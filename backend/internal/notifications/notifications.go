package notifications

import (
	"database/sql"
	"time"

	"github.com/google/uuid"
	"github.com/yasinatby/swappy/backend/internal/models"
)

type Service struct {
	db *sql.DB
}

func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

func (s *Service) Create(userID, notifType, title, body, entityType, entityID string) (*models.Notification, error) {
	n := &models.Notification{
		ID:         uuid.New().String(),
		UserID:     userID,
		Type:       notifType,
		Title:      title,
		Body:       body,
		EntityType: entityType,
		EntityID:   entityID,
		IsRead:     false,
		CreatedAt:  time.Now(),
	}

	query := `
	INSERT INTO notifications (id, user_id, type, title, body, entity_type, entity_id, is_read, created_at)
	VALUES (?, ?, ?, ?, ?, ?, ?, 0, ?)
	`
	_, err := s.db.Exec(query, n.ID, n.UserID, n.Type, n.Title, n.Body, n.EntityType, n.EntityID, n.CreatedAt)
	if err != nil {
		return nil, err
	}

	return n, nil
}

func (s *Service) List(userID string, unreadOnly bool) ([]models.Notification, error) {
	query := `
	SELECT id, user_id, type, title, body, entity_type, entity_id, is_read, created_at
	FROM notifications
	WHERE user_id = ?
	`
	if unreadOnly {
		query += " AND is_read = 0"
	}
	query += " ORDER BY created_at DESC LIMIT 50"

	rows, err := s.db.Query(query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var notifs []models.Notification
	for rows.Next() {
		var n models.Notification
		var isReadInt int
		var entityType, entityID sql.NullString
		if err := rows.Scan(&n.ID, &n.UserID, &n.Type, &n.Title, &n.Body, &entityType, &entityID, &isReadInt, &n.CreatedAt); err != nil {
			return nil, err
		}
		n.IsRead = isReadInt > 0
		n.EntityType = entityType.String
		n.EntityID = entityID.String
		notifs = append(notifs, n)
	}

	return notifs, nil
}

func (s *Service) MarkAsRead(id, userID string) error {
	_, err := s.db.Exec("UPDATE notifications SET is_read = 1 WHERE id = ? AND user_id = ?", id, userID)
	return err
}

func (s *Service) MarkAllAsRead(userID string) error {
	_, err := s.db.Exec("UPDATE notifications SET is_read = 1 WHERE user_id = ?", userID)
	return err
}

func (s *Service) GetUnreadCount(userID string) (int, error) {
	var count int
	err := s.db.QueryRow("SELECT COUNT(1) FROM notifications WHERE user_id = ? AND is_read = 0", userID).Scan(&count)
	return count, err
}
