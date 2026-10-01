package admin

import (
	"database/sql"
	"errors"
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

func (s *Service) GetStats() (*models.AdminStats, error) {
	var stats models.AdminStats

	_ = s.db.QueryRow("SELECT COUNT(1) FROM users").Scan(&stats.TotalUsers)
	_ = s.db.QueryRow("SELECT COUNT(1) FROM items WHERE status = 'available'").Scan(&stats.ActiveItems)
	_ = s.db.QueryRow("SELECT COUNT(1) FROM swaps WHERE status = 'completed'").Scan(&stats.CompletedTrades)
	_ = s.db.QueryRow("SELECT COUNT(1) FROM reports WHERE status = 'open'").Scan(&stats.OpenReports)
	_ = s.db.QueryRow("SELECT COUNT(1) FROM users WHERE is_banned = 1").Scan(&stats.BannedUsers)
	_ = s.db.QueryRow("SELECT COALESCE(SUM(estimated_value), 0.0) FROM items WHERE status = 'traded'").Scan(&stats.TotalEstimatedVolume)

	return &stats, nil
}

func (s *Service) CreateReport(reporterID string, req models.CreateReportRequest) (*models.Report, error) {
	if req.Reason == "" {
		return nil, errors.New("report reason is required")
	}

	rep := &models.Report{
		ID:           uuid.New().String(),
		ReporterID:   reporterID,
		TargetUserID: req.TargetUserID,
		TargetItemID: req.TargetItemID,
		Reason:       req.Reason,
		Description:  req.Description,
		Status:       "open",
		CreatedAt:    time.Now(),
	}

	query := `
	INSERT INTO reports (id, reporter_id, target_user_id, target_item_id, reason, description, status, created_at)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`
	_, err := s.db.Exec(query,
		rep.ID, rep.ReporterID, rep.TargetUserID, rep.TargetItemID,
		rep.Reason, rep.Description, rep.Status, rep.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	return rep, nil
}

func (s *Service) ListReports(status string) ([]models.Report, error) {
	query := `
	SELECT id, reporter_id, target_user_id, target_item_id, reason, description, status, created_at
	FROM reports
	`
	var args []any
	if status != "" {
		query += " WHERE status = ?"
		args = append(args, status)
	}
	query += " ORDER BY created_at DESC"

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var reports []models.Report
	for rows.Next() {
		var rep models.Report
		var targetUser, targetItem, desc sql.NullString
		if err := rows.Scan(&rep.ID, &rep.ReporterID, &targetUser, &targetItem, &rep.Reason, &desc, &rep.Status, &rep.CreatedAt); err != nil {
			return nil, err
		}
		rep.TargetUserID = targetUser.String
		rep.TargetItemID = targetItem.String
		rep.Description = desc.String
		reports = append(reports, rep)
	}

	return reports, nil
}

func (s *Service) ResolveReport(reportID, status string) error {
	if status != "resolved" && status != "dismissed" {
		status = "resolved"
	}
	res, err := s.db.Exec("UPDATE reports SET status = ? WHERE id = ?", status, reportID)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err != nil || rows == 0 {
		return errors.New("report not found")
	}
	return nil
}

func (s *Service) BanUser(userID string) error {
	_, err := s.db.Exec("UPDATE users SET is_banned = 1 WHERE id = ?", userID)
	return err
}

func (s *Service) UnbanUser(userID string) error {
	_, err := s.db.Exec("UPDATE users SET is_banned = 0 WHERE id = ?", userID)
	return err
}

func (s *Service) ModerateItem(itemID, newStatus string) error {
	_, err := s.db.Exec("UPDATE items SET status = ? WHERE id = ?", newStatus, itemID)
	return err
}
