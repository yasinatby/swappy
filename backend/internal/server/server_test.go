package server_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/yasinatby/swappy/backend/internal/config"
	"github.com/yasinatby/swappy/backend/internal/database"
	"github.com/yasinatby/swappy/backend/internal/models"
	"github.com/yasinatby/swappy/backend/internal/server"
)

type testRunner struct {
	handler http.Handler
}

func setupTestRunner(t *testing.T) *testRunner {
	t.Helper()
	cfg := config.Config{
		Host:               "127.0.0.1",
		Port:               "8080",
		DBPath:             ":memory:",
		JWTSecret:          "test-secret-key-32-characters-long!",
		UploadDir:          t.TempDir(),
		CORSAllowedOrigins: "*",
	}

	db, err := database.Connect(cfg.DBPath)
	if err != nil {
		t.Fatalf("failed to connect db: %v", err)
	}

	handler, err := server.NewHandler(cfg, db)
	if err != nil {
		t.Fatalf("failed to create handler: %v", err)
	}

	t.Cleanup(func() {
		db.Close()
	})

	return &testRunner{handler: handler}
}

func (tr *testRunner) do(t *testing.T, method, path string, token string, body any) (*http.Response, []byte) {
	t.Helper()
	var bodyReader *bytes.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("failed to marshal request: %v", err)
		}
		bodyReader = bytes.NewReader(data)
	} else {
		bodyReader = bytes.NewReader([]byte{})
	}

	req := httptest.NewRequest(method, path, bodyReader)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	rec := httptest.NewRecorder()
	tr.handler.ServeHTTP(rec, req)

	res := rec.Result()
	return res, rec.Body.Bytes()
}

func TestCompleteSwappyFlow(t *testing.T) {
	tr := setupTestRunner(t)

	// 1. Health check
	resp, body := tr.do(t, "GET", "/health", "", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 health, got %d: %s", resp.StatusCode, string(body))
	}

	// 2. Register Max and Lisa
	maxReg := models.RegisterRequest{
		Username:  "max",
		Email:     "max@swappy.de",
		Password:  "password123",
		FirstName: "Max",
		City:      "Heilbronn",
		Latitude:  49.1427,
		Longitude: 9.2109,
	}
	resp, body = tr.do(t, "POST", "/api/auth/register", "", maxReg)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("failed to register max: %d %s", resp.StatusCode, string(body))
	}
	var maxAuth models.AuthResponse
	_ = json.Unmarshal(body, &maxAuth)

	lisaReg := models.RegisterRequest{
		Username:  "lisa",
		Email:     "lisa@swappy.de",
		Password:  "password123",
		FirstName: "Lisa",
		City:      "Stuttgart",
		Latitude:  48.7758,
		Longitude: 9.1829,
	}
	resp, body = tr.do(t, "POST", "/api/auth/register", "", lisaReg)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("failed to register lisa: %d %s", resp.StatusCode, string(body))
	}
	var lisaAuth models.AuthResponse
	_ = json.Unmarshal(body, &lisaAuth)

	// 3. Test Rule 4: Max tries to propose a swap BEFORE creating an item!
	// Lisa first creates her iPhone item
	lisaItemReq := models.ItemCreateRequest{
		Title:          "iPhone 15",
		Description:    "iPhone 15 128GB in schwarz, top Zustand",
		Category:       "Elektronik",
		Condition:      "Sehr gut",
		EstimatedValue: 600.0,
		TradeInterests: []string{"PlayStation 5", "PS5", "Gaming"},
		City:           "Stuttgart",
	}
	resp, body = tr.do(t, "POST", "/api/items", lisaAuth.Token, lisaItemReq)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("failed to create lisa item: %d %s", resp.StatusCode, string(body))
	}
	var lisaItem models.Item
	_ = json.Unmarshal(body, &lisaItem)

	// Max has NO items published. Max tries to propose swap on Lisa's item:
	invalidProposal := models.SwapProposalRequest{
		TargetItemID:   lisaItem.ID,
		OfferedItemIDs: []string{"some-dummy-id"},
		Message:        "Tauschversuch ohne eigenes Objekt",
	}
	resp, body = tr.do(t, "POST", "/api/swaps", maxAuth.Token, invalidProposal)
	if resp.StatusCode == http.StatusCreated {
		t.Fatalf("RULE 4 VIOLATION: user without active items was allowed to propose swap!")
	}
	t.Logf("Rule 4 successfully enforced: Max was rejected because he has no active items (status: %d, response: %s)", resp.StatusCode, string(body))

	// 4. Max lists his PlayStation 5
	maxItemReq := models.ItemCreateRequest{
		Title:          "PlayStation 5",
		Description:    "PS5 mit 2 Controllern, sehr guter Zustand",
		Category:       "Gaming",
		Condition:      "Sehr gut",
		EstimatedValue: 450.0,
		TradeInterests: []string{"iPhone 15", "iPhone", "Smartphone"},
		City:           "Heilbronn",
	}
	resp, body = tr.do(t, "POST", "/api/items", maxAuth.Token, maxItemReq)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("failed to create max item: %d %s", resp.StatusCode, string(body))
	}
	var maxItem models.Item
	_ = json.Unmarshal(body, &maxItem)

	// 5. Test Smart Swap Matching: Max wants iPhone, Lisa wants PS5!
	resp, body = tr.do(t, "GET", "/api/swaps/matches", maxAuth.Token, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("failed to get matches: %d %s", resp.StatusCode, string(body))
	}
	var matches []models.SwapMatch
	_ = json.Unmarshal(body, &matches)
	if len(matches) == 0 {
		t.Fatalf("expected smart match between Max and Lisa, got 0 matches")
	}
	if matches[0].MatchScore < 80 {
		t.Fatalf("expected high match score for perfect match, got %d", matches[0].MatchScore)
	}
	t.Logf("Smart match verified: %s (Score: %d)", matches[0].MatchReason, matches[0].MatchScore)

	// 6. Test Reverse Trade Interest Search ("Ich suche ...")
	// Search for items where owner wants "PS5"
	resp, body = tr.do(t, "GET", "/api/items?trade_interest=PS5", "", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("failed trade interest search: %d", resp.StatusCode)
	}
	var searchResults []models.Item
	_ = json.Unmarshal(body, &searchResults)
	if len(searchResults) == 0 || searchResults[0].ID != lisaItem.ID {
		t.Fatalf("expected Lisa's iPhone in reverse trade interest search for 'PS5'")
	}

	// 7. Max proposes swap: his PS5 + 50 € for Lisa's iPhone 15
	proposal := models.SwapProposalRequest{
		TargetItemID:   lisaItem.ID,
		OfferedItemIDs: []string{maxItem.ID},
		CashAdjustment: 50.0,
		Message:        "Ich würde gerne meine PS5 + 50 € gegen dein iPhone tauschen.",
		HandoverMethod: "personal",
	}
	resp, body = tr.do(t, "POST", "/api/swaps", maxAuth.Token, proposal)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("failed to propose swap: %d %s", resp.StatusCode, string(body))
	}
	var createdSwap models.Swap
	_ = json.Unmarshal(body, &createdSwap)
	if createdSwap.Status != "pending" {
		t.Fatalf("expected status pending, got %s", createdSwap.Status)
	}

	// 8. Lisa counters the offer: asks for 100 € cash adjustment
	counterReq := models.SwapCounterRequest{
		OfferedItemIDs: []string{maxItem.ID},
		CashAdjustment: 100.0,
		Message:        "Ich würde die PS5 gegen mein iPhone + 100 € tauschen.",
	}
	resp, body = tr.do(t, "POST", "/api/swaps/"+createdSwap.ID+"/counter", lisaAuth.Token, counterReq)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("failed to counter swap: %d %s", resp.StatusCode, string(body))
	}
	var counteredSwap models.Swap
	_ = json.Unmarshal(body, &counteredSwap)
	if counteredSwap.Status != "countered" || counteredSwap.CashAdjustment != 100.0 {
		t.Fatalf("expected countered status with 100 cash adjustment, got %v", counteredSwap)
	}

	// 9. Max accepts the counter offer
	resp, body = tr.do(t, "POST", "/api/swaps/"+createdSwap.ID+"/accept", maxAuth.Token, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("failed to accept swap: %d %s", resp.StatusCode, string(body))
	}
	var agreedSwap models.Swap
	_ = json.Unmarshal(body, &agreedSwap)
	if agreedSwap.Status != "agreed" {
		t.Fatalf("expected agreed status, got %s", agreedSwap.Status)
	}

	// Verify items are now marked 'requested' 🟡
	resp, body = tr.do(t, "GET", "/api/items/"+lisaItem.ID, "", nil)
	var checkedTarget models.Item
	_ = json.Unmarshal(body, &checkedTarget)
	if checkedTarget.Status != "requested" {
		t.Fatalf("expected item status requested, got %s", checkedTarget.Status)
	}

	// 10. Chat messages
	chatReq := models.SendMessageRequest{
		Text: "Super! Wann passt es dir für die Übergabe in Heilbronn?",
	}
	resp, body = tr.do(t, "POST", "/api/swaps/"+createdSwap.ID+"/messages", maxAuth.Token, chatReq)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("failed to send message: %d %s", resp.StatusCode, string(body))
	}

	resp, body = tr.do(t, "GET", "/api/swaps/"+createdSwap.ID+"/messages", lisaAuth.Token, nil)
	var msgs []models.Message
	_ = json.Unmarshal(body, &msgs)
	if len(msgs) == 0 {
		t.Fatalf("expected messages, got 0")
	}

	// 11. Complete swap
	// Max confirms completion
	resp, _ = tr.do(t, "POST", "/api/swaps/"+createdSwap.ID+"/complete", maxAuth.Token, nil)
	// Lisa confirms completion
	resp, body = tr.do(t, "POST", "/api/swaps/"+createdSwap.ID+"/complete", lisaAuth.Token, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("failed to complete swap: %d %s", resp.StatusCode, string(body))
	}
	var completedSwap models.Swap
	_ = json.Unmarshal(body, &completedSwap)
	if completedSwap.Status != "completed" {
		t.Fatalf("expected completed status, got %s", completedSwap.Status)
	}

	// Verify items are now 'traded' 🔴
	resp, body = tr.do(t, "GET", "/api/items/"+lisaItem.ID, "", nil)
	_ = json.Unmarshal(body, &checkedTarget)
	if checkedTarget.Status != "traded" {
		t.Fatalf("expected item status traded, got %s", checkedTarget.Status)
	}

	// 12. Rate user
	revReq := models.CreateReviewRequest{
		Rating:        5,
		Reliability:   5,
		Communication: 5,
		ItemCondition: 5,
		Adherence:     5,
		Comment:       "Sehr zuverlässiger Tauschpartner. Alles wie beschrieben!",
	}
	resp, body = tr.do(t, "POST", "/api/swaps/"+createdSwap.ID+"/reviews", maxAuth.Token, revReq)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("failed to submit review: %d %s", resp.StatusCode, string(body))
	}

	// Check Lisa's profile: completed trades count = 1, rating = 5.0
	resp, body = tr.do(t, "GET", "/api/users/"+lisaAuth.User.ID, "", nil)
	var lisaProfile models.UserPublicProfile
	_ = json.Unmarshal(body, &lisaProfile)
	if lisaProfile.CompletedTradesCount != 1 {
		t.Fatalf("expected 1 completed trade, got %d", lisaProfile.CompletedTradesCount)
	}
	if lisaProfile.RatingAverage != 5.0 {
		t.Fatalf("expected rating average 5.0, got %f", lisaProfile.RatingAverage)
	}

	// 13. Favorites test
	resp, body = tr.do(t, "POST", "/api/items/"+maxItem.ID+"/favorite", lisaAuth.Token, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("failed to favorite item: %d %s", resp.StatusCode, string(body))
	}

	// 14. Notifications test
	resp, body = tr.do(t, "GET", "/api/notifications", lisaAuth.Token, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("failed to get notifications: %d %s", resp.StatusCode, string(body))
	}
	var notifs []models.Notification
	_ = json.Unmarshal(body, &notifs)
	if len(notifs) == 0 {
		t.Fatalf("expected notifications for lisa, got 0")
	}

	// 15. Admin Stats test
	adminLogin := models.LoginRequest{
		UsernameOrEmail: "admin",
		Password:        "admin1234",
	}
	resp, body = tr.do(t, "POST", "/api/auth/login", "", adminLogin)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("failed admin login: %d %s", resp.StatusCode, string(body))
	}
	var adminAuth models.AuthResponse
	_ = json.Unmarshal(body, &adminAuth)

	resp, body = tr.do(t, "GET", "/api/admin/stats", adminAuth.Token, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("failed to get admin stats: %d %s", resp.StatusCode, string(body))
	}
	var stats models.AdminStats
	_ = json.Unmarshal(body, &stats)
	if stats.CompletedTrades != 1 {
		t.Fatalf("expected 1 completed trade in admin stats, got %d", stats.CompletedTrades)
	}
	t.Logf("Full integration flow passed! Completed trades: %d, Users: %d", stats.CompletedTrades, stats.TotalUsers)
}
