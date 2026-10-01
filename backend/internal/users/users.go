package users

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/yasinatby/swappy/backend/internal/auth"
	"github.com/yasinatby/swappy/backend/internal/models"
)

type Service struct {
	db *sql.DB
}

func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

func (s *Service) Register(req models.RegisterRequest) (*models.User, error) {
	req.Username = strings.TrimSpace(req.Username)
	req.Email = strings.TrimSpace(strings.ToLower(req.Email))

	if req.Username == "" || req.Email == "" || req.Password == "" {
		return nil, errors.New("username, email and password are required")
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	user := &models.User{
		ID:           uuid.New().String(),
		Username:     req.Username,
		Email:        req.Email,
		PasswordHash: hash,
		FirstName:    strings.TrimSpace(req.FirstName),
		City:         strings.TrimSpace(req.City),
		Latitude:     req.Latitude,
		Longitude:    req.Longitude,
		Role:         models.RoleUser,
		IsVerified:   false,
		IsBanned:     false,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	query := `
	INSERT INTO users (id, username, email, password_hash, first_name, city, latitude, longitude, role, is_verified, is_banned, created_at, updated_at)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	_, err = s.db.Exec(query,
		user.ID, user.Username, user.Email, user.PasswordHash,
		user.FirstName, user.City, user.Latitude, user.Longitude,
		user.Role, user.IsVerified, user.IsBanned, user.CreatedAt, user.UpdatedAt,
	)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return nil, errors.New("username or email already in use")
		}
		return nil, fmt.Errorf("failed to insert user: %w", err)
	}

	return user, nil
}

func (s *Service) Login(usernameOrEmail, password string) (*models.User, error) {
	usernameOrEmail = strings.TrimSpace(strings.ToLower(usernameOrEmail))
	if usernameOrEmail == "" || password == "" {
		return nil, errors.New("username/email and password are required")
	}

	query := `
	SELECT id, username, email, password_hash, first_name, avatar_url, bio, city, postal_code,
	       latitude, longitude, is_verified, is_banned, role, completed_trades_count,
	       rating_average, rating_count, created_at, updated_at
	FROM users
	WHERE LOWER(username) = ? OR LOWER(email) = ?
	`
	row := s.db.QueryRow(query, usernameOrEmail, usernameOrEmail)

	var u models.User
	var firstName, avatarURL, bio, city, postalCode sql.NullString

	err := row.Scan(
		&u.ID, &u.Username, &u.Email, &u.PasswordHash, &firstName, &avatarURL, &bio, &city, &postalCode,
		&u.Latitude, &u.Longitude, &u.IsVerified, &u.IsBanned, &u.Role, &u.CompletedTradesCount,
		&u.RatingAverage, &u.RatingCount, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("invalid credentials")
		}
		return nil, err
	}

	if u.IsBanned {
		return nil, errors.New("this account has been banned")
	}

	if !auth.CheckPassword(password, u.PasswordHash) {
		return nil, errors.New("invalid credentials")
	}

	u.FirstName = firstName.String
	u.AvatarURL = avatarURL.String
	u.Bio = bio.String
	u.City = city.String
	u.PostalCode = postalCode.String

	return &u, nil
}

func (s *Service) GetByID(id string) (*models.User, error) {
	query := `
	SELECT id, username, email, password_hash, first_name, avatar_url, bio, city, postal_code,
	       latitude, longitude, is_verified, is_banned, role, completed_trades_count,
	       rating_average, rating_count, created_at, updated_at
	FROM users
	WHERE id = ?
	`
	row := s.db.QueryRow(query, id)

	var u models.User
	var firstName, avatarURL, bio, city, postalCode sql.NullString

	err := row.Scan(
		&u.ID, &u.Username, &u.Email, &u.PasswordHash, &firstName, &avatarURL, &bio, &city, &postalCode,
		&u.Latitude, &u.Longitude, &u.IsVerified, &u.IsBanned, &u.Role, &u.CompletedTradesCount,
		&u.RatingAverage, &u.RatingCount, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	u.FirstName = firstName.String
	u.AvatarURL = avatarURL.String
	u.Bio = bio.String
	u.City = city.String
	u.PostalCode = postalCode.String

	return &u, nil
}

func (s *Service) GetPublicProfile(id string) (*models.UserPublicProfile, error) {
	u, err := s.GetByID(id)
	if err != nil {
		return nil, err
	}

	pub := u.ToPublic()
	return &pub, nil
}

func (s *Service) UpdateProfile(id string, req models.UpdateProfileRequest) (*models.User, error) {
	current, err := s.GetByID(id)
	if err != nil {
		return nil, err
	}

	if req.FirstName != nil {
		current.FirstName = *req.FirstName
	}
	if req.Bio != nil {
		current.Bio = *req.Bio
	}
	if req.AvatarURL != nil {
		current.AvatarURL = *req.AvatarURL
	}
	if req.City != nil {
		current.City = *req.City
	}
	if req.PostalCode != nil {
		current.PostalCode = *req.PostalCode
	}
	if req.Latitude != nil {
		current.Latitude = *req.Latitude
	}
	if req.Longitude != nil {
		current.Longitude = *req.Longitude
	}
	current.UpdatedAt = time.Now()

	query := `
	UPDATE users
	SET first_name = ?, bio = ?, avatar_url = ?, city = ?, postal_code = ?, latitude = ?, longitude = ?, updated_at = ?
	WHERE id = ?
	`
	_, err = s.db.Exec(query,
		current.FirstName, current.Bio, current.AvatarURL, current.City, current.PostalCode,
		current.Latitude, current.Longitude, current.UpdatedAt, current.ID,
	)
	if err != nil {
		return nil, err
	}

	return current, nil
}

func (s *Service) BlockUser(blockerID, blockedID string) error {
	if blockerID == blockedID {
		return errors.New("cannot block yourself")
	}

	query := `INSERT OR IGNORE INTO blocks (blocker_id, blocked_id, created_at) VALUES (?, ?, ?)`
	_, err := s.db.Exec(query, blockerID, blockedID, time.Now())
	return err
}

func (s *Service) UnblockUser(blockerID, blockedID string) error {
	query := `DELETE FROM blocks WHERE blocker_id = ? AND blocked_id = ?`
	_, err := s.db.Exec(query, blockerID, blockedID)
	return err
}

func (s *Service) IsBlocked(userA, userB string) (bool, error) {
	query := `
	SELECT COUNT(1) FROM blocks
	WHERE (blocker_id = ? AND blocked_id = ?) OR (blocker_id = ? AND blocked_id = ?)
	`
	var count int
	err := s.db.QueryRow(query, userA, userB, userB, userA).Scan(&count)
	return count > 0, err
}

func (s *Service) GetBlockedUsers(userID string) ([]models.UserPublicProfile, error) {
	query := `
	SELECT u.id, u.username, u.first_name, u.avatar_url, u.bio, u.city,
	       u.completed_trades_count, u.rating_average, u.rating_count, u.is_verified, u.created_at
	FROM users u
	INNER JOIN blocks b ON b.blocked_id = u.id
	WHERE b.blocker_id = ?
	`
	rows, err := s.db.Query(query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var profiles []models.UserPublicProfile
	for rows.Next() {
		var p models.UserPublicProfile
		var firstName, avatarURL, bio, city sql.NullString
		if err := rows.Scan(&p.ID, &p.Username, &firstName, &avatarURL, &bio, &city,
			&p.CompletedTradesCount, &p.RatingAverage, &p.RatingCount, &p.IsVerified, &p.CreatedAt); err != nil {
			return nil, err
		}
		p.FirstName = firstName.String
		p.AvatarURL = avatarURL.String
		p.Bio = bio.String
		p.City = city.String
		profiles = append(profiles, p)
	}

	return profiles, nil
}

func (s *Service) SearchUsers(q string, limit int) ([]models.UserPublicProfile, error) {
	if limit <= 0 {
		limit = 20
	}
	like := "%" + strings.ToLower(q) + "%"

	query := `
	SELECT id, username, first_name, avatar_url, bio, city, completed_trades_count, rating_average, rating_count, is_verified, created_at
	FROM users
	WHERE (LOWER(username) LIKE ? OR LOWER(first_name) LIKE ?) AND is_banned = 0
	LIMIT ?
	`
	rows, err := s.db.Query(query, like, like, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []models.UserPublicProfile
	for rows.Next() {
		var p models.UserPublicProfile
		var firstName, avatarURL, bio, city sql.NullString
		if err := rows.Scan(&p.ID, &p.Username, &firstName, &avatarURL, &bio, &city,
			&p.CompletedTradesCount, &p.RatingAverage, &p.RatingCount, &p.IsVerified, &p.CreatedAt); err != nil {
			return nil, err
		}
		p.FirstName = firstName.String
		p.AvatarURL = avatarURL.String
		p.Bio = bio.String
		p.City = city.String
		results = append(results, p)
	}

	return results, nil
}

func (s *Service) EnsureAdminUser(adminEmail, adminUsername, adminPassword string) error {
	var count int
	err := s.db.QueryRow("SELECT COUNT(1) FROM users WHERE role = 'admin'").Scan(&count)
	if err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	hash, err := auth.HashPassword(adminPassword)
	if err != nil {
		return err
	}

	id := uuid.New().String()
	now := time.Now()
	_, err = s.db.Exec(`
	INSERT INTO users (id, username, email, password_hash, first_name, role, is_verified, is_banned, created_at, updated_at)
	VALUES (?, ?, ?, ?, 'Admin', 'admin', 1, 0, ?, ?)
	`, id, adminUsername, adminEmail, hash, now, now)

	return err
}
