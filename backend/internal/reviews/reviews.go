package reviews

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/yasinatby/swappy/backend/internal/models"
	"github.com/yasinatby/swappy/backend/internal/notifications"
	"github.com/yasinatby/swappy/backend/internal/users"
)

type Service struct {
	db           *sql.DB
	userService  *users.Service
	notifService *notifications.Service
}

func NewService(db *sql.DB, userService *users.Service, notifService *notifications.Service) *Service {
	return &Service{
		db:           db,
		userService:  userService,
		notifService: notifService,
	}
}

func (s *Service) Create(swapID, reviewerID string, req models.CreateReviewRequest) (*models.Review, error) {
	if req.Rating < 1 || req.Rating > 5 {
		return nil, errors.New("rating must be between 1 and 5")
	}

	var status, initiatorID, recipientID string
	err := s.db.QueryRow("SELECT status, initiator_id, recipient_id FROM swaps WHERE id = ?", swapID).Scan(&status, &initiatorID, &recipientID)
	if err != nil {
		return nil, errors.New("swap not found")
	}

	if status != models.SwapStatusCompleted {
		return nil, errors.New("reviews can only be submitted for completed swaps")
	}

	if reviewerID != initiatorID && reviewerID != recipientID {
		return nil, errors.New("unauthorized: not a participant of this swap")
	}

	var targetID string
	if reviewerID == initiatorID {
		targetID = recipientID
	} else {
		targetID = initiatorID
	}

	// Check if already reviewed
	var count int
	err = s.db.QueryRow("SELECT COUNT(1) FROM reviews WHERE swap_id = ? AND reviewer_id = ?", swapID, reviewerID).Scan(&count)
	if err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, errors.New("you have already reviewed this swap")
	}

	reviewID := uuid.New().String()
	now := time.Now()

	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	query := `
	INSERT INTO reviews (
		id, swap_id, reviewer_id, target_id, rating,
		reliability, communication, item_condition, adherence, comment, created_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	_, err = tx.Exec(query,
		reviewID, swapID, reviewerID, targetID, req.Rating,
		req.Reliability, req.Communication, req.ItemCondition, req.Adherence,
		req.Comment, now,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to insert review: %w", err)
	}

	// Recalculate rating average and count for target user
	var avgRating float64
	var totalReviews int
	row := tx.QueryRow(`
		SELECT COALESCE(AVG(rating), 0.0), COUNT(1)
		FROM reviews
		WHERE target_id = ?
	`, targetID)
	if err := row.Scan(&avgRating, &totalReviews); err != nil {
		return nil, err
	}

	_, err = tx.Exec(`
		UPDATE users
		SET rating_average = ?, rating_count = ?
		WHERE id = ?
	`, avgRating, totalReviews, targetID)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	reviewerProfile, _ := s.userService.GetPublicProfile(reviewerID)

	// Send notification
	reviewerName := reviewerID
	if reviewerProfile != nil {
		reviewerName = reviewerProfile.Username
	}
	_, _ = s.notifService.Create(
		targetID,
		models.NotifReviewReceived,
		"Neue Bewertung erhalten! ⭐",
		fmt.Sprintf("%s hat dir eine %d-Sterne Bewertung hinterlassen.", reviewerName, req.Rating),
		"swap",
		swapID,
	)

	return &models.Review{
		ID:            reviewID,
		SwapID:        swapID,
		ReviewerID:    reviewerID,
		TargetID:      targetID,
		Rating:        req.Rating,
		Reliability:   req.Reliability,
		Communication: req.Communication,
		ItemCondition: req.ItemCondition,
		Adherence:     req.Adherence,
		Comment:       req.Comment,
		Reviewer:      reviewerProfile,
		CreatedAt:     now,
	}, nil
}

func (s *Service) GetByUserID(targetUserID string) ([]models.Review, error) {
	query := `
	SELECT r.id, r.swap_id, r.reviewer_id, r.target_id, r.rating,
	       r.reliability, r.communication, r.item_condition, r.adherence, r.comment, r.created_at,
	       u.id, u.username, u.first_name, u.avatar_url, u.bio, u.city,
	       u.completed_trades_count, u.rating_average, u.rating_count, u.is_verified, u.created_at
	FROM reviews r
	JOIN users u ON u.id = r.reviewer_id
	WHERE r.target_id = ?
	ORDER BY r.created_at DESC
	`
	rows, err := s.db.Query(query, targetUserID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var reviewsList []models.Review
	for rows.Next() {
		var rev models.Review
		var comment sql.NullString
		var u models.UserPublicProfile
		var uFirstName, uAvatar, uBio, uCity sql.NullString

		err := rows.Scan(
			&rev.ID, &rev.SwapID, &rev.ReviewerID, &rev.TargetID, &rev.Rating,
			&rev.Reliability, &rev.Communication, &rev.ItemCondition, &rev.Adherence,
			&comment, &rev.CreatedAt,
			&u.ID, &u.Username, &uFirstName, &uAvatar, &uBio, &uCity,
			&u.CompletedTradesCount, &u.RatingAverage, &u.RatingCount, &u.IsVerified, &u.CreatedAt,
		)
		if err != nil {
			return nil, err
		}

		rev.Comment = comment.String
		u.FirstName = uFirstName.String
		u.AvatarURL = uAvatar.String
		u.Bio = uBio.String
		u.City = uCity.String
		rev.Reviewer = &u

		reviewsList = append(reviewsList, rev)
	}

	return reviewsList, nil
}
