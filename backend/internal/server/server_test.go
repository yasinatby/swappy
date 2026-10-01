package server_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/yasinatby/swappy/backend/internal/auth"
	"github.com/yasinatby/swappy/backend/internal/config"
	"github.com/yasinatby/swappy/backend/internal/items"
	"github.com/yasinatby/swappy/backend/internal/server"
	"github.com/yasinatby/swappy/backend/internal/storage"
	"github.com/yasinatby/swappy/backend/internal/swaps"
	"github.com/yasinatby/swappy/backend/internal/testdb"
	"github.com/yasinatby/swappy/backend/internal/users"
)

type client struct {
	t     *testing.T
	base  string
	token string
}

func newServer(t *testing.T, devLogin bool) string {
	pool := testdb.New(t)
	store, err := storage.NewLocal(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	swapService := swaps.NewService(pool)
	srv := server.New(config.Config{DevLogin: devLogin}, server.Deps{
		Tokens:  auth.NewTokens("test-secret", time.Hour),
		Items:   items.NewService(pool),
		Swaps:   swapService,
		Users:   users.NewService(pool, swapService),
		Storage: store,
	})
	ts := httptest.NewServer(srv.Handler)
	t.Cleanup(ts.Close)
	return ts.URL
}

func (c *client) do(method, path string, body any) (int, map[string]any) {
	c.t.Helper()
	var reader io.Reader
	if b, ok := body.([]byte); ok {
		reader = bytes.NewReader(b)
	} else if body != nil {
		data, _ := json.Marshal(body)
		reader = bytes.NewReader(data)
	}
	req, _ := http.NewRequest(method, c.base+path, reader)
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		c.t.Fatal(err)
	}
	defer resp.Body.Close()
	var out map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return resp.StatusCode, out
}

func login(t *testing.T, base, name string) *client {
	t.Helper()
	c := &client{t: t, base: base}
	status, body := c.do("POST", "/v1/dev/login", map[string]string{"display_name": name})
	if status != 200 {
		t.Fatalf("dev login: %d %v", status, body)
	}
	c.token = body["token"].(string)
	return c
}

func errorCode(body map[string]any) string {
	e, _ := body["error"].(map[string]any)
	code, _ := e["code"].(string)
	return code
}

func expect(t *testing.T, gotStatus int, body map[string]any, wantStatus int, wantCode string) {
	t.Helper()
	if gotStatus != wantStatus || errorCode(body) != wantCode {
		t.Fatalf("got %d %q (%v), want %d %q", gotStatus, errorCode(body), body, wantStatus, wantCode)
	}
}

func (c *client) createItem(title string) string {
	c.t.Helper()
	status, body := c.do("POST", "/v1/items", map[string]any{
		"title": title, "category": "sonstiges", "condition": "good", "publish": true,
	})
	if status != 201 {
		c.t.Fatalf("create item: %d %v", status, body)
	}
	return body["id"].(string)
}

func TestDevLoginIsOffByDefault(t *testing.T) {
	base := newServer(t, false)
	c := &client{t: t, base: base}
	status, _ := c.do("POST", "/v1/dev/login", map[string]string{"display_name": "anna"})
	if status != http.StatusNotFound {
		t.Fatalf("dev login reachable without SWAPPY_DEV_LOGIN: %d", status)
	}
}

func TestAuthErrors(t *testing.T) {
	base := newServer(t, true)
	anon := &client{t: t, base: base}
	status, body := anon.do("GET", "/v1/me", nil)
	expect(t, status, body, 401, "unauthorized")

	forged := &client{t: t, base: base, token: auth.NewTokens("wrong", time.Hour).Issue("00000000-0000-4000-8000-00000000000a")}
	status, body = forged.do("GET", "/v1/items", nil)
	expect(t, status, body, 401, "unauthorized")

	status, body = anon.do("POST", "/v1/dev/login", map[string]string{"display_name": "x"})
	expect(t, status, body, 400, "invalid_input")
}

func TestValidationErrors(t *testing.T) {
	base := newServer(t, true)
	anna := login(t, base, "anna")

	status, body := anna.do("POST", "/v1/items", []byte("{not json"))
	expect(t, status, body, 400, "invalid_json")
	status, body = anna.do("POST", "/v1/items", map[string]any{"title": "", "category": "sonstiges", "condition": "good"})
	expect(t, status, body, 400, "invalid_input")
	status, body = anna.do("POST", "/v1/items", map[string]any{"title": "x", "category": "geld", "condition": "good"})
	expect(t, status, body, 400, "invalid_input")
	status, body = anna.do("GET", "/v1/items/not-a-uuid", nil)
	expect(t, status, body, 404, "item_not_found")
	status, body = anna.do("POST", "/v1/offers/00000000-0000-4000-8000-000000000000/explode", nil)
	expect(t, status, body, 404, "not_found")

	id := anna.createItem("Lampe")
	status, body = anna.do("POST", "/v1/items/"+id+"/images", []byte("GIF89a not a jpeg"))
	expect(t, status, body, 400, "invalid_image")
}

func TestOfferFlowOverHTTP(t *testing.T) {
	base := newServer(t, true)
	anna, ben := login(t, base, "anna"), login(t, base, "ben")

	// Invariant 4 at the API: without an active item, no offer.
	target := anna.createItem("Rennrad")
	status, body := ben.do("POST", "/v1/offers", map[string]any{"target_item_id": target, "offered_item_ids": []string{}})
	expect(t, status, body, 400, "invalid_input")

	draftStatus, draft := ben.do("POST", "/v1/items", map[string]any{"title": "Entwurf", "category": "sonstiges", "condition": "good"})
	if draftStatus != 201 {
		t.Fatal(draft)
	}
	status, body = ben.do("POST", "/v1/offers", map[string]any{"target_item_id": target, "offered_item_ids": []string{draft["id"].(string)}})
	expect(t, status, body, 409, "no_active_item")

	guitar := ben.createItem("Gitarre")
	status, offer := ben.do("POST", "/v1/offers", map[string]any{"target_item_id": target, "offered_item_ids": []string{guitar}})
	if status != 201 || offer["status"] != "pending" {
		t.Fatalf("create offer: %d %v", status, offer)
	}
	offerID := offer["id"].(string)

	status, body = ben.do("POST", "/v1/offers/"+offerID+"/accept", nil)
	expect(t, status, body, 403, "not_allowed")

	status, body = anna.do("POST", "/v1/offers/"+offerID+"/accept", nil)
	if status != 200 || body["status"] != "accepted" {
		t.Fatalf("accept: %d %v", status, body)
	}
	status, body = anna.do("DELETE", "/v1/items/"+target, nil)
	expect(t, status, body, 409, "item_locked")

	_, item := anna.do("GET", "/v1/items/"+target, nil)
	if item["status"] != "reserved" {
		t.Fatalf("target is %v", item["status"])
	}
}

// Kontolöschung in der App (App Store 1.2): soft delete, offers cancelled,
// token stops working.
func TestDeleteAccount(t *testing.T) {
	base := newServer(t, true)
	anna, ben := login(t, base, "anna"), login(t, base, "ben")
	target := anna.createItem("Rennrad")
	_, offer := ben.do("POST", "/v1/offers", map[string]any{"target_item_id": target, "offered_item_ids": []string{ben.createItem("Gitarre")}})
	anna.do("POST", "/v1/offers/"+offer["id"].(string)+"/accept", nil)

	status, _ := anna.do("DELETE", "/v1/me", nil)
	if status != 204 {
		t.Fatalf("delete account: %d", status)
	}
	status, body := anna.do("GET", "/v1/me", nil)
	expect(t, status, body, 401, "unauthorized")

	_, got := ben.do("GET", "/v1/offers/"+offer["id"].(string), nil)
	if got["status"] != "cancelled" {
		t.Fatalf("offer is %v after account deletion", got["status"])
	}
	_, feed := ben.do("GET", "/v1/items", nil)
	if n := len(feed["items"].([]any)); n != 0 {
		t.Fatalf("deleted user's items still in feed: %d", n)
	}
	// The name is free again; a new dev login creates a fresh account.
	fresh := login(t, base, "anna")
	_, profile := fresh.do("GET", "/v1/me", nil)
	if profile["active_items"].(float64) != 0 {
		t.Fatalf("fresh account inherited items: %v", profile)
	}
}
