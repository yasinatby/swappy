package chat

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/yasinatby/swappy/backend/internal/models"
	"github.com/yasinatby/swappy/backend/internal/notifications"
	"github.com/yasinatby/swappy/backend/internal/users"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for dev / mobile client
	},
}

type Client struct {
	Hub    *Hub
	Conn   *websocket.Conn
	Send   chan models.Message
	UserID string
	SwapID string
}

type Hub struct {
	clients    map[string]map[*Client]bool // map[swapID]map[client]true
	register   chan *Client
	unregister chan *Client
	broadcast  chan models.Message
	mu         sync.RWMutex
}

func NewHub() *Hub {
	return &Hub{
		clients:    make(map[string]map[*Client]bool),
		register:   make(chan *Client, 32),
		unregister: make(chan *Client, 32),
		broadcast:  make(chan models.Message, 256),
	}
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			if _, ok := h.clients[client.SwapID]; !ok {
				h.clients[client.SwapID] = make(map[*Client]bool)
			}
			h.clients[client.SwapID][client] = true
			h.mu.Unlock()

		case client := <-h.unregister:
			h.mu.Lock()
			if clients, ok := h.clients[client.SwapID]; ok {
				if _, ok := clients[client]; ok {
					delete(clients, client)
					close(client.Send)
					if len(clients) == 0 {
						delete(h.clients, client.SwapID)
					}
				}
			}
			h.mu.Unlock()

		case msg := <-h.broadcast:
			h.mu.RLock()
			if clients, ok := h.clients[msg.SwapID]; ok {
				for client := range clients {
					select {
					case client.Send <- msg:
					default:
						close(client.Send)
						delete(clients, client)
					}
				}
			}
			h.mu.RUnlock()
		}
	}
}

type Service struct {
	db           *sql.DB
	userService  *users.Service
	notifService *notifications.Service
	Hub          *Hub
}

func NewService(db *sql.DB, userService *users.Service, notifService *notifications.Service) *Service {
	hub := NewHub()
	go hub.Run()

	return &Service{
		db:           db,
		userService:  userService,
		notifService: notifService,
		Hub:          hub,
	}
}

func (s *Service) SendMessage(swapID, senderID string, req models.SendMessageRequest) (*models.Message, error) {
	if req.Text == "" && req.ImageURL == "" {
		return nil, errors.New("message cannot be empty")
	}

	// Verify user is part of the swap
	var initiatorID, recipientID string
	err := s.db.QueryRow("SELECT initiator_id, recipient_id FROM swaps WHERE id = ?", swapID).Scan(&initiatorID, &recipientID)
	if err != nil {
		return nil, errors.New("swap not found")
	}

	if senderID != initiatorID && senderID != recipientID {
		return nil, errors.New("unauthorized: not a participant of this swap")
	}

	var recipientUser string
	if senderID == initiatorID {
		recipientUser = recipientID
	} else {
		recipientUser = initiatorID
	}

	// Check blocks
	if blocked, _ := s.userService.IsBlocked(senderID, recipientUser); blocked {
		return nil, errors.New("cannot send message to this user")
	}

	msgID := uuid.New().String()
	now := time.Now()

	query := `
	INSERT INTO messages (id, swap_id, sender_id, text, image_url, is_system, created_at)
	VALUES (?, ?, ?, ?, ?, 0, ?)
	`
	_, err = s.db.Exec(query, msgID, swapID, senderID, req.Text, req.ImageURL, now)
	if err != nil {
		return nil, err
	}

	senderProfile, _ := s.userService.GetPublicProfile(senderID)

	msg := models.Message{
		ID:        msgID,
		SwapID:    swapID,
		SenderID:  senderID,
		Sender:    senderProfile,
		Text:      req.Text,
		ImageURL:  req.ImageURL,
		IsSystem:  false,
		CreatedAt: now,
	}

	// Broadcast over WebSocket hub
	select {
	case s.Hub.broadcast <- msg:
	default:
	}

	// Send in-app notification to receiver
	senderName := senderID
	if senderProfile != nil {
		senderName = senderProfile.Username
	}
	body := req.Text
	if body == "" && req.ImageURL != "" {
		body = "📷 [Bild gesendet]"
	}
	_, _ = s.notifService.Create(
		recipientUser,
		models.NotifNewMessage,
		fmt.Sprintf("Neue Nachricht von %s 💬", senderName),
		body,
		"swap",
		swapID,
	)

	return &msg, nil
}

func (s *Service) GetMessages(swapID, userID string) ([]models.Message, error) {
	// Verify user is part of swap
	var initiatorID, recipientID string
	err := s.db.QueryRow("SELECT initiator_id, recipient_id FROM swaps WHERE id = ?", swapID).Scan(&initiatorID, &recipientID)
	if err != nil {
		return nil, errors.New("swap not found")
	}

	if userID != initiatorID && userID != recipientID {
		return nil, errors.New("unauthorized: not a participant of this swap")
	}

	query := `
	SELECT m.id, m.swap_id, m.sender_id, m.text, m.image_url, m.is_system, m.created_at,
	       u.id, u.username, u.first_name, u.avatar_url, u.bio, u.city,
	       u.completed_trades_count, u.rating_average, u.rating_count, u.is_verified, u.created_at
	FROM messages m
	LEFT JOIN users u ON u.id = m.sender_id
	WHERE m.swap_id = ?
	ORDER BY m.created_at ASC
	`
	rows, err := s.db.Query(query, swapID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []models.Message
	for rows.Next() {
		var m models.Message
		var text, img sql.NullString
		var isSysInt int
		var u models.UserPublicProfile
		var uFirstName, uAvatar, uBio, uCity sql.NullString
		var uCompleted, uRatingCount, uVerified int
		var uRatingAvg float64
		var uCreatedAt time.Time

		err := rows.Scan(
			&m.ID, &m.SwapID, &m.SenderID, &text, &img, &isSysInt, &m.CreatedAt,
			&u.ID, &u.Username, &uFirstName, &uAvatar, &uBio, &uCity,
			&uCompleted, &uRatingAvg, &uRatingCount, &uVerified, &uCreatedAt,
		)
		if err != nil {
			return nil, err
		}

		m.Text = text.String
		m.ImageURL = img.String
		m.IsSystem = isSysInt > 0

		if u.ID != "" {
			u.FirstName = uFirstName.String
			u.AvatarURL = uAvatar.String
			u.Bio = uBio.String
			u.City = uCity.String
			u.CompletedTradesCount = uCompleted
			u.RatingAverage = uRatingAvg
			u.RatingCount = uRatingCount
			u.IsVerified = uVerified > 0
			u.CreatedAt = uCreatedAt
			m.Sender = &u
		}

		messages = append(messages, m)
	}

	return messages, nil
}

func (s *Service) HandleWebSocket(w http.ResponseWriter, r *http.Request, swapID, userID string) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	client := &Client{
		Hub:    s.Hub,
		Conn:   conn,
		Send:   make(chan models.Message, 256),
		UserID: userID,
		SwapID: swapID,
	}

	client.Hub.register <- client

	// Writer goroutine
	go func() {
		defer func() {
			conn.Close()
		}()
		for message := range client.Send {
			if err := conn.WriteJSON(message); err != nil {
				break
			}
		}
	}()

	// Reader goroutine
	go func() {
		defer func() {
			client.Hub.unregister <- client
			conn.Close()
		}()
		for {
			var req models.SendMessageRequest
			if err := conn.ReadJSON(&req); err != nil {
				break
			}
			_, _ = s.SendMessage(swapID, userID, req)
		}
	}()
}
