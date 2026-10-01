package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/yasinatby/swappy/backend/internal/apperr"
	"github.com/yasinatby/swappy/backend/internal/auth"
	"github.com/yasinatby/swappy/backend/internal/config"
	"github.com/yasinatby/swappy/backend/internal/items"
	"github.com/yasinatby/swappy/backend/internal/storage"
	"github.com/yasinatby/swappy/backend/internal/swaps"
	"github.com/yasinatby/swappy/backend/internal/users"
)

type Deps struct {
	Tokens  auth.Tokens
	Items   *items.Service
	Swaps   *swaps.Service
	Users   *users.Service
	Storage *storage.Local
}

type api struct {
	Deps
}

var (
	errUnauthorized = apperr.New(apperr.Unauthorized, "unauthorized", "missing or invalid token")
	errBadJSON      = apperr.New(apperr.Invalid, "invalid_json", "request body is not valid JSON")
	errNotFound     = apperr.New(apperr.NotFound, "not_found", "not found")
)

func New(appConfig config.Config, deps Deps) *http.Server {
	a := &api{Deps: deps}
	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", healthHandler)
	mux.HandleFunc("GET /v1/images/{name}", a.serveImage)

	if appConfig.DevLogin {
		log.Printf("WARNING: dev login is enabled — never run this configuration in production")
		mux.HandleFunc("GET /v1/dev/profiles", a.devProfiles)
		mux.HandleFunc("POST /v1/dev/login", a.devLogin)
	}

	authed := func(pattern string, h http.HandlerFunc) {
		mux.Handle(pattern, a.requireUser(h))
	}
	authed("GET /v1/me", a.getMe)
	authed("PUT /v1/me", a.updateMe)
	authed("DELETE /v1/me", a.deleteMe)
	authed("GET /v1/me/items", a.myItems)
	authed("GET /v1/me/blocks", a.myBlocks)

	authed("GET /v1/items", a.feed)
	authed("POST /v1/items", a.createItem)
	authed("GET /v1/items/{id}", a.getItem)
	authed("PUT /v1/items/{id}", a.updateItem)
	authed("DELETE /v1/items/{id}", a.deleteItem)
	authed("POST /v1/items/{id}/publish", a.publishItem)
	authed("POST /v1/items/{id}/images", a.uploadImage)
	authed("DELETE /v1/items/{id}/images/{imageID}", a.deleteImage)

	authed("GET /v1/users/{id}", a.getUser)
	authed("POST /v1/users/{id}/block", a.block)
	authed("DELETE /v1/users/{id}/block", a.unblock)
	authed("POST /v1/reports", a.report)

	authed("GET /v1/offers", a.listOffers)
	authed("POST /v1/offers", a.createOffer)
	authed("GET /v1/offers/{id}", a.getOffer)
	authed("POST /v1/offers/{id}/{action}", a.offerAction)

	authed("GET /v1/notifications", a.notifications)
	authed("POST /v1/notifications/read", a.readNotifications)

	return &http.Server{
		Addr:              appConfig.Address(),
		Handler:           logRequests(mux),
		ReadHeaderTimeout: 10 * time.Second,
	}
}

func healthHandler(response http.ResponseWriter, _ *http.Request) {
	response.WriteHeader(http.StatusOK)
	_, _ = response.Write([]byte(`{"status":"ok"}`))
}

// requireUser accepts "Authorization: Bearer <token>" for a user whose
// profile still exists and is not deleted.
func (a *api) requireUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		if !ok {
			writeError(w, errUnauthorized)
			return
		}
		userID, err := a.Tokens.Verify(token)
		if err != nil {
			writeError(w, errUnauthorized)
			return
		}
		active, err := a.Users.IsActive(r.Context(), userID)
		if err != nil {
			writeError(w, err)
			return
		}
		if !active {
			writeError(w, errUnauthorized)
			return
		}
		next.ServeHTTP(w, r.WithContext(auth.WithUserID(r.Context(), userID)))
	})
}

func me(r *http.Request) string { return auth.UserID(r.Context()) }

func readJSON(r *http.Request, dst any) error {
	body := io.LimitReader(r.Body, 64<<10)
	if err := json.NewDecoder(body).Decode(dst); err != nil {
		return errBadJSON
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, err error) {
	var appErr *apperr.Error
	if !errors.As(err, &appErr) {
		if errors.Is(err, context.Canceled) {
			return
		}
		log.Printf("internal error: %v", err)
		appErr = apperr.New(-1, "internal", "something went wrong")
	}
	status := map[apperr.Kind]int{
		apperr.Invalid:      http.StatusBadRequest,
		apperr.NotFound:     http.StatusNotFound,
		apperr.Forbidden:    http.StatusForbidden,
		apperr.Conflict:     http.StatusConflict,
		apperr.Unauthorized: http.StatusUnauthorized,
	}[appErr.Kind]
	if status == 0 {
		status = http.StatusInternalServerError
	}
	writeJSON(w, status, map[string]any{"error": map[string]string{"code": appErr.Code, "message": appErr.Message}})
}

// respond writes v (or an empty 204) unless err is set.
func respond(w http.ResponseWriter, status int, v any, err error) {
	if err != nil {
		writeError(w, err)
		return
	}
	if v == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	writeJSON(w, status, v)
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

// logRequests logs method, route pattern, status and duration — never bodies,
// tokens or other personal data.
func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		log.Printf("%s %d %s", r.Pattern, rec.status, time.Since(start).Round(time.Millisecond))
	})
}
