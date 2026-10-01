package server

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"github.com/yasinatby/swappy/backend/internal/admin"
	"github.com/yasinatby/swappy/backend/internal/auth"
	"github.com/yasinatby/swappy/backend/internal/chat"
	"github.com/yasinatby/swappy/backend/internal/config"
	"github.com/yasinatby/swappy/backend/internal/database"
	"github.com/yasinatby/swappy/backend/internal/items"
	"github.com/yasinatby/swappy/backend/internal/matching"
	"github.com/yasinatby/swappy/backend/internal/models"
	"github.com/yasinatby/swappy/backend/internal/notifications"
	"github.com/yasinatby/swappy/backend/internal/reviews"
	"github.com/yasinatby/swappy/backend/internal/storage"
	"github.com/yasinatby/swappy/backend/internal/swaps"
	"github.com/yasinatby/swappy/backend/internal/users"
)

type AppServer struct {
	Config       config.Config
	DB           *sql.DB
	Users        *users.Service
	Items        *items.Service
	Swaps        *swaps.Service
	Chat         *chat.Service
	Reviews      *reviews.Service
	Notifs       *notifications.Service
	Matching     *matching.Service
	Admin        *admin.Service
	Storage      *storage.Storage
}

// Handler returns the fully configured http.Handler
func NewHandler(cfg config.Config, db *sql.DB) (http.Handler, error) {
	strg, err := storage.New(cfg.UploadDir)
	if err != nil {
		return nil, err
	}

	userService := users.NewService(db)
	notifService := notifications.NewService(db)
	itemService := items.NewService(db)
	chatService := chat.NewService(db, userService, notifService)
	reviewService := reviews.NewService(db, userService, notifService)
	matchingService := matching.NewService(itemService)
	adminService := admin.NewService(db)

	// Ensure admin user
	_ = userService.EnsureAdminUser("admin@swappy.local", "admin", "admin1234")

	// We use the swaps package
	swapService := swaps.NewService(db, itemService, userService, notifService)

	s := &AppServer{
		Config:   cfg,
		DB:       db,
		Users:    userService,
		Items:    itemService,
		Swaps:    swapService,
		Chat:     chatService,
		Reviews:  reviewService,
		Notifs:   notifService,
		Matching: matchingService,
		Admin:    adminService,
		Storage:  strg,
	}

	mux := http.NewServeMux()

	// Static & Uploads
	mux.Handle("GET /uploads/", http.StripPrefix("/uploads/", http.FileServer(http.Dir(cfg.UploadDir))))

	// Health
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "app": "swappy"})
	})

	// Auth Endpoints
	mux.HandleFunc("POST /api/auth/register", s.handleRegister)
	mux.HandleFunc("POST /api/auth/login", s.handleLogin)
	mux.HandleFunc("GET /api/auth/me", s.requireAuth(s.handleGetMe))

	// User & Profile Endpoints
	mux.HandleFunc("GET /api/users/{id}", s.handleGetUserProfile)
	mux.HandleFunc("PUT /api/users/profile", s.requireAuth(s.handleUpdateProfile))
	mux.HandleFunc("POST /api/users/{id}/block", s.requireAuth(s.handleBlockUser))
	mux.HandleFunc("POST /api/users/{id}/unblock", s.requireAuth(s.handleUnblockUser))
	mux.HandleFunc("GET /api/users/blocked", s.requireAuth(s.handleGetBlockedUsers))
	mux.HandleFunc("GET /api/users/search", s.handleSearchUsers)

	// Items Endpoints
	mux.HandleFunc("POST /api/items", s.requireAuth(s.handleCreateItem))
	mux.HandleFunc("GET /api/items/{id}", s.optionalAuth(s.handleGetItem))
	mux.HandleFunc("PUT /api/items/{id}", s.requireAuth(s.handleUpdateItem))
	mux.HandleFunc("DELETE /api/items/{id}", s.requireAuth(s.handleDeleteItem))
	mux.HandleFunc("GET /api/items", s.optionalAuth(s.handleListItems))
	mux.HandleFunc("POST /api/items/{id}/favorite", s.requireAuth(s.handleToggleFavorite))
	mux.HandleFunc("GET /api/favorites", s.requireAuth(s.handleGetFavorites))
	mux.HandleFunc("GET /api/feed/home", s.optionalAuth(s.handleGetHomeFeed))

	// Swaps & Barter Endpoints
	mux.HandleFunc("POST /api/swaps", s.requireAuth(s.handleProposeSwap))
	mux.HandleFunc("GET /api/swaps", s.requireAuth(s.handleListSwaps))
	mux.HandleFunc("GET /api/swaps/{id}", s.requireAuth(s.handleGetSwap))
	mux.HandleFunc("POST /api/swaps/{id}/counter", s.requireAuth(s.handleCounterSwap))
	mux.HandleFunc("POST /api/swaps/{id}/accept", s.requireAuth(s.handleAcceptSwap))
	mux.HandleFunc("POST /api/swaps/{id}/reject", s.requireAuth(s.handleRejectSwap))
	mux.HandleFunc("POST /api/swaps/{id}/complete", s.requireAuth(s.handleCompleteSwap))
	mux.HandleFunc("GET /api/swaps/matches", s.requireAuth(s.handleGetSwapMatches))

	// Chat Endpoints
	mux.HandleFunc("GET /api/swaps/{id}/messages", s.requireAuth(s.handleGetMessages))
	mux.HandleFunc("POST /api/swaps/{id}/messages", s.requireAuth(s.handleSendMessage))
	mux.HandleFunc("GET /api/swaps/{id}/ws", s.handleWebSocket)

	// Reviews Endpoints
	mux.HandleFunc("POST /api/swaps/{id}/reviews", s.requireAuth(s.handleCreateReview))
	mux.HandleFunc("GET /api/users/{id}/reviews", s.handleGetUserReviews)

	// Notifications Endpoints
	mux.HandleFunc("GET /api/notifications", s.requireAuth(s.handleGetNotifications))
	mux.HandleFunc("POST /api/notifications/{id}/read", s.requireAuth(s.handleMarkNotificationRead))
	mux.HandleFunc("POST /api/notifications/read-all", s.requireAuth(s.handleMarkAllNotificationsRead))
	mux.HandleFunc("GET /api/notifications/unread-count", s.requireAuth(s.handleGetUnreadNotificationCount))

	// Safety & Reports
	mux.HandleFunc("POST /api/reports", s.requireAuth(s.handleCreateReport))

	// Admin Endpoints
	mux.HandleFunc("GET /api/admin/stats", s.requireAdmin(s.handleAdminStats))
	mux.HandleFunc("GET /api/admin/reports", s.requireAdmin(s.handleAdminReports))
	mux.HandleFunc("POST /api/admin/reports/{id}/resolve", s.requireAdmin(s.handleAdminResolveReport))
	mux.HandleFunc("POST /api/admin/users/{id}/ban", s.requireAdmin(s.handleAdminBanUser))
	mux.HandleFunc("POST /api/admin/users/{id}/unban", s.requireAdmin(s.handleAdminUnbanUser))
	mux.HandleFunc("POST /api/admin/items/{id}/moderate", s.requireAdmin(s.handleAdminModerateItem))

	// Upload Endpoints
	mux.HandleFunc("POST /api/upload", s.requireAuth(s.handleUpload))

	// Wrap with CORS middleware
	return s.corsMiddleware(mux), nil
}

func New(appConfig config.Config) *http.Server {
	db, err := database.Connect(appConfig.DBPath)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}

	handler, err := NewHandler(appConfig, db)
	if err != nil {
		log.Fatalf("failed to initialize handler: %v", err)
	}

	return &http.Server{
		Addr:    appConfig.Address(),
		Handler: handler,
	}
}

// Helpers

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("error writing json response: %v", err)
	}
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

// Auth Handlers

func (s *AppServer) handleRegister(w http.ResponseWriter, r *http.Request) {
	var req models.RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	user, err := s.Users.Register(req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	token, err := auth.GenerateToken(user.ID, user.Username, user.Role, s.Config.JWTSecret)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate authentication token")
		return
	}

	writeJSON(w, http.StatusCreated, models.AuthResponse{
		Token: token,
		User:  *user,
	})
}

func (s *AppServer) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req models.LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	user, err := s.Users.Login(req.UsernameOrEmail, req.Password)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err.Error())
		return
	}

	token, err := auth.GenerateToken(user.ID, user.Username, user.Role, s.Config.JWTSecret)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate authentication token")
		return
	}

	writeJSON(w, http.StatusOK, models.AuthResponse{
		Token: token,
		User:  *user,
	})
}

func (s *AppServer) handleGetMe(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.GetUserID(r.Context())
	user, err := s.Users.GetByID(userID)
	if err != nil {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}
	writeJSON(w, http.StatusOK, user)
}

func (s *AppServer) handleGetUserProfile(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	profile, err := s.Users.GetPublicProfile(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "user profile not found")
		return
	}

	// Fetch active and traded items for this user
	activeItems, _ := s.Items.List(items.Filter{UserID: id, Status: models.ItemStatusAvailable}, "")
	tradedItems, _ := s.Items.List(items.Filter{UserID: id, Status: models.ItemStatusTraded}, "")
	profile.ActiveItems = activeItems
	profile.TradedItems = tradedItems

	writeJSON(w, http.StatusOK, profile)
}

func (s *AppServer) handleUpdateProfile(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.GetUserID(r.Context())
	var req models.UpdateProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	user, err := s.Users.UpdateProfile(userID, req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, user)
}

func (s *AppServer) handleBlockUser(w http.ResponseWriter, r *http.Request) {
	blockerID, _ := auth.GetUserID(r.Context())
	targetID := r.PathValue("id")

	if err := s.Users.BlockUser(blockerID, targetID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "user blocked successfully"})
}

func (s *AppServer) handleUnblockUser(w http.ResponseWriter, r *http.Request) {
	blockerID, _ := auth.GetUserID(r.Context())
	targetID := r.PathValue("id")

	if err := s.Users.UnblockUser(blockerID, targetID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "user unblocked successfully"})
}

func (s *AppServer) handleGetBlockedUsers(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.GetUserID(r.Context())
	blocked, err := s.Users.GetBlockedUsers(userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if blocked == nil {
		blocked = []models.UserPublicProfile{}
	}
	writeJSON(w, http.StatusOK, blocked)
}

func (s *AppServer) handleSearchUsers(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	results, err := s.Users.SearchUsers(q, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if results == nil {
		results = []models.UserPublicProfile{}
	}
	writeJSON(w, http.StatusOK, results)
}

// Items Handlers

func (s *AppServer) handleCreateItem(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.GetUserID(r.Context())
	var req models.ItemCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	item, err := s.Items.Create(userID, req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, item)
}

func (s *AppServer) handleGetItem(w http.ResponseWriter, r *http.Request) {
	itemID := r.PathValue("id")
	currentUserID, _ := auth.GetUserID(r.Context())

	item, err := s.Items.GetByID(itemID, currentUserID)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, item)
}

func (s *AppServer) handleUpdateItem(w http.ResponseWriter, r *http.Request) {
	itemID := r.PathValue("id")
	userID, _ := auth.GetUserID(r.Context())

	var req models.ItemUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	item, err := s.Items.Update(itemID, userID, req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, item)
}

func (s *AppServer) handleDeleteItem(w http.ResponseWriter, r *http.Request) {
	itemID := r.PathValue("id")
	userID, _ := auth.GetUserID(r.Context())

	if err := s.Items.Delete(itemID, userID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "item deleted successfully"})
}

func (s *AppServer) handleListItems(w http.ResponseWriter, r *http.Request) {
	currentUserID, _ := auth.GetUserID(r.Context())
	q := r.URL.Query()

	filter := items.Filter{
		Query:               q.Get("q"),
		TradeInterestSearch: q.Get("trade_interest"),
		Category:            q.Get("category"),
		Condition:           q.Get("condition"),
		Status:              q.Get("status"),
		UserID:              q.Get("user_id"),
		Sort:                q.Get("sort"),
	}

	if minVal := q.Get("min_value"); minVal != "" {
		if val, err := strconv.ParseFloat(minVal, 64); err == nil {
			filter.MinEstimatedValue = &val
		}
	}
	if maxVal := q.Get("max_value"); maxVal != "" {
		if val, err := strconv.ParseFloat(maxVal, 64); err == nil {
			filter.MaxEstimatedValue = &val
		}
	}
	if latStr := q.Get("lat"); latStr != "" {
		if lat, err := strconv.ParseFloat(latStr, 64); err == nil {
			filter.Latitude = &lat
		}
	}
	if lngStr := q.Get("lng"); lngStr != "" {
		if lng, err := strconv.ParseFloat(lngStr, 64); err == nil {
			filter.Longitude = &lng
		}
	}
	if distStr := q.Get("max_dist"); distStr != "" {
		if dist, err := strconv.ParseFloat(distStr, 64); err == nil {
			filter.MaxDistanceKm = &dist
		}
	}
	if limitStr := q.Get("limit"); limitStr != "" {
		filter.Limit, _ = strconv.Atoi(limitStr)
	}
	if offsetStr := q.Get("offset"); offsetStr != "" {
		filter.Offset, _ = strconv.Atoi(offsetStr)
	}

	list, err := s.Items.List(filter, currentUserID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if list == nil {
		list = []models.Item{}
	}

	writeJSON(w, http.StatusOK, list)
}

func (s *AppServer) handleToggleFavorite(w http.ResponseWriter, r *http.Request) {
	itemID := r.PathValue("id")
	userID, _ := auth.GetUserID(r.Context())

	favorited, err := s.Items.ToggleFavorite(userID, itemID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if favorited {
		// Send notification to item owner
		if item, err := s.Items.GetByID(itemID, userID); err == nil && item.UserID != userID {
			user, _ := s.Users.GetPublicProfile(userID)
			uName := "Jemand"
			if user != nil {
				uName = user.Username
			}
			_, _ = s.Notifs.Create(
				item.UserID,
				models.NotifItemFavorited,
				"Objekt favorisiert! ❤️",
				uName+" hat dein Objekt '"+item.Title+"' zu den Favoriten hinzugefügt.",
				"item",
				itemID,
			)
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"item_id":      itemID,
		"is_favorited": favorited,
	})
}

func (s *AppServer) handleGetFavorites(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.GetUserID(r.Context())
	favorites, err := s.Items.GetFavorites(userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if favorites == nil {
		favorites = []models.Item{}
	}
	writeJSON(w, http.StatusOK, favorites)
}

func (s *AppServer) handleGetHomeFeed(w http.ResponseWriter, r *http.Request) {
	currentUserID, _ := auth.GetUserID(r.Context())
	lat, _ := strconv.ParseFloat(r.URL.Query().Get("lat"), 64)
	lng, _ := strconv.ParseFloat(r.URL.Query().Get("lng"), 64)

	feed, err := s.Items.GetHomeFeed(currentUserID, lat, lng)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, feed)
}

// Swaps Handlers

func (s *AppServer) handleProposeSwap(w http.ResponseWriter, r *http.Request) {
	initiatorID, _ := auth.GetUserID(r.Context())
	var req models.SwapProposalRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	swap, err := s.Swaps.Propose(initiatorID, req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, swap)
}

func (s *AppServer) handleListSwaps(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.GetUserID(r.Context())
	status := r.URL.Query().Get("status")

	swaps, err := s.Swaps.ListUserSwaps(userID, status)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if swaps == nil {
		swaps = []models.Swap{}
	}

	writeJSON(w, http.StatusOK, swaps)
}

func (s *AppServer) handleGetSwap(w http.ResponseWriter, r *http.Request) {
	swapID := r.PathValue("id")
	userID, _ := auth.GetUserID(r.Context())

	swap, err := s.Swaps.GetByID(swapID, userID)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, swap)
}

func (s *AppServer) handleCounterSwap(w http.ResponseWriter, r *http.Request) {
	swapID := r.PathValue("id")
	userID, _ := auth.GetUserID(r.Context())

	var req models.SwapCounterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	swap, err := s.Swaps.Counter(swapID, userID, req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, swap)
}

func (s *AppServer) handleAcceptSwap(w http.ResponseWriter, r *http.Request) {
	swapID := r.PathValue("id")
	userID, _ := auth.GetUserID(r.Context())

	swap, err := s.Swaps.Accept(swapID, userID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, swap)
}

func (s *AppServer) handleRejectSwap(w http.ResponseWriter, r *http.Request) {
	swapID := r.PathValue("id")
	userID, _ := auth.GetUserID(r.Context())

	swap, err := s.Swaps.Reject(swapID, userID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, swap)
}

func (s *AppServer) handleCompleteSwap(w http.ResponseWriter, r *http.Request) {
	swapID := r.PathValue("id")
	userID, _ := auth.GetUserID(r.Context())

	swap, err := s.Swaps.Complete(swapID, userID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, swap)
}

func (s *AppServer) handleGetSwapMatches(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.GetUserID(r.Context())
	matches, err := s.Matching.FindMatches(userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if matches == nil {
		matches = []models.SwapMatch{}
	}

	writeJSON(w, http.StatusOK, matches)
}

// Chat Handlers

func (s *AppServer) handleGetMessages(w http.ResponseWriter, r *http.Request) {
	swapID := r.PathValue("id")
	userID, _ := auth.GetUserID(r.Context())

	messages, err := s.Chat.GetMessages(swapID, userID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if messages == nil {
		messages = []models.Message{}
	}

	writeJSON(w, http.StatusOK, messages)
}

func (s *AppServer) handleSendMessage(w http.ResponseWriter, r *http.Request) {
	swapID := r.PathValue("id")
	userID, _ := auth.GetUserID(r.Context())

	var req models.SendMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	msg, err := s.Chat.SendMessage(swapID, userID, req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, msg)
}

func (s *AppServer) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	swapID := r.PathValue("id")
	token := r.URL.Query().Get("token")
	if token == "" {
		writeError(w, http.StatusUnauthorized, "token parameter required")
		return
	}

	claims, err := auth.ParseToken(token, s.Config.JWTSecret)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid token")
		return
	}

	s.Chat.HandleWebSocket(w, r, swapID, claims.UserID)
}

// Review Handlers

func (s *AppServer) handleCreateReview(w http.ResponseWriter, r *http.Request) {
	swapID := r.PathValue("id")
	reviewerID, _ := auth.GetUserID(r.Context())

	var req models.CreateReviewRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	review, err := s.Reviews.Create(swapID, reviewerID, req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, review)
}

func (s *AppServer) handleGetUserReviews(w http.ResponseWriter, r *http.Request) {
	targetUserID := r.PathValue("id")
	reviewsList, err := s.Reviews.GetByUserID(targetUserID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if reviewsList == nil {
		reviewsList = []models.Review{}
	}

	writeJSON(w, http.StatusOK, reviewsList)
}

// Notification Handlers

func (s *AppServer) handleGetNotifications(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.GetUserID(r.Context())
	unreadOnly := r.URL.Query().Get("unread") == "true"

	notifs, err := s.Notifs.List(userID, unreadOnly)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if notifs == nil {
		notifs = []models.Notification{}
	}

	writeJSON(w, http.StatusOK, notifs)
}

func (s *AppServer) handleMarkNotificationRead(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	userID, _ := auth.GetUserID(r.Context())

	if err := s.Notifs.MarkAsRead(id, userID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "marked as read"})
}

func (s *AppServer) handleMarkAllNotificationsRead(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.GetUserID(r.Context())

	if err := s.Notifs.MarkAllAsRead(userID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "all notifications marked as read"})
}

func (s *AppServer) handleGetUnreadNotificationCount(w http.ResponseWriter, r *http.Request) {
	userID, _ := auth.GetUserID(r.Context())

	count, err := s.Notifs.GetUnreadCount(userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]int{"unread_count": count})
}

// Reports & Safety Handlers

func (s *AppServer) handleCreateReport(w http.ResponseWriter, r *http.Request) {
	reporterID, _ := auth.GetUserID(r.Context())
	var req models.CreateReportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	report, err := s.Admin.CreateReport(reporterID, req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, report)
}

// Admin Handlers

func (s *AppServer) handleAdminStats(w http.ResponseWriter, r *http.Request) {
	stats, err := s.Admin.GetStats()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, stats)
}

func (s *AppServer) handleAdminReports(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	reports, err := s.Admin.ListReports(status)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if reports == nil {
		reports = []models.Report{}
	}
	writeJSON(w, http.StatusOK, reports)
}

func (s *AppServer) handleAdminResolveReport(w http.ResponseWriter, r *http.Request) {
	reportID := r.PathValue("id")
	status := r.URL.Query().Get("status")

	if err := s.Admin.ResolveReport(reportID, status); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "report updated"})
}

func (s *AppServer) handleAdminBanUser(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("id")
	if err := s.Admin.BanUser(userID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "user banned"})
}

func (s *AppServer) handleAdminUnbanUser(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("id")
	if err := s.Admin.UnbanUser(userID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "user unbanned"})
}

func (s *AppServer) handleAdminModerateItem(w http.ResponseWriter, r *http.Request) {
	itemID := r.PathValue("id")
	status := r.URL.Query().Get("status")
	if status == "" {
		status = models.ItemStatusDeactivated
	}

	if err := s.Admin.ModerateItem(itemID, status); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "item status updated"})
}

// Upload Handler

func (s *AppServer) handleUpload(w http.ResponseWriter, r *http.Request) {
	// 10MB max upload
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "file too large or invalid multipart form")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "file field required")
		return
	}
	defer file.Close()

	urlPath, err := s.Storage.SaveFile(file, header)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"url": urlPath})
}

// Middleware Helpers

func (s *AppServer) requireAuth(fn http.HandlerFunc) http.HandlerFunc {
	return auth.RequireAuth(s.Config.JWTSecret)(fn).ServeHTTP
}

func (s *AppServer) optionalAuth(fn http.HandlerFunc) http.HandlerFunc {
	return auth.OptionalAuth(s.Config.JWTSecret)(fn).ServeHTTP
}

func (s *AppServer) requireAdmin(fn http.HandlerFunc) http.HandlerFunc {
	return auth.RequireAdmin(s.Config.JWTSecret)(fn).ServeHTTP
}

func (s *AppServer) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", s.Config.CORSAllowedOrigins)
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}
