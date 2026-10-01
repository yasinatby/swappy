package items

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/yasinatby/swappy/backend/internal/models"
)

type Filter struct {
	Query               string
	TradeInterestSearch string // "Ich suche..." search
	Category            string
	Condition           string
	MinEstimatedValue   *float64
	MaxEstimatedValue   *float64
	Status              string // default "available"
	UserID              string
	Latitude            *float64
	Longitude           *float64
	MaxDistanceKm       *float64
	Sort                string // "newest", "popular", "value_asc", "value_desc", "distance"
	Limit               int
	Offset              int
}

type Service struct {
	db *sql.DB
}

func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

func (s *Service) Create(userID string, req models.ItemCreateRequest) (*models.Item, error) {
	req.Title = strings.TrimSpace(req.Title)
	if req.Title == "" {
		return nil, errors.New("title is required")
	}
	if req.Category == "" {
		return nil, errors.New("category is required")
	}
	if req.Condition == "" {
		return nil, errors.New("condition is required")
	}
	if req.EstimatedValue <= 0 {
		return nil, errors.New("estimated value must be greater than 0")
	}

	tradeInterestsJSON, err := json.Marshal(req.TradeInterests)
	if err != nil {
		tradeInterestsJSON = []byte("[]")
	}

	imagesJSON, err := json.Marshal(req.Images)
	if err != nil {
		imagesJSON = []byte("[]")
	}

	now := time.Now()
	item := &models.Item{
		ID:             uuid.New().String(),
		UserID:         userID,
		Title:          req.Title,
		Description:    strings.TrimSpace(req.Description),
		Category:       req.Category,
		Condition:      req.Condition,
		EstimatedValue: req.EstimatedValue,
		MinSwapValue:   req.MinSwapValue,
		TradeInterests: req.TradeInterests,
		Images:         req.Images,
		City:           strings.TrimSpace(req.City),
		Latitude:       req.Latitude,
		Longitude:      req.Longitude,
		Status:         models.ItemStatusAvailable,
		ViewsCount:     0,
		FavoritesCount: 0,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	query := `
	INSERT INTO items (
		id, user_id, title, description, category, condition, estimated_value,
		min_swap_value, trade_interests, images, city, latitude, longitude,
		status, views_count, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 0, ?, ?)
	`
	_, err = s.db.Exec(query,
		item.ID, item.UserID, item.Title, item.Description, item.Category, item.Condition,
		item.EstimatedValue, item.MinSwapValue, string(tradeInterestsJSON), string(imagesJSON),
		item.City, item.Latitude, item.Longitude, item.Status, item.CreatedAt, item.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create item: %w", err)
	}

	return item, nil
}

func (s *Service) GetByID(id string, currentUserID string) (*models.Item, error) {
	// Increment view count
	_, _ = s.db.Exec("UPDATE items SET views_count = views_count + 1 WHERE id = ?", id)

	query := `
	SELECT i.id, i.user_id, i.title, i.description, i.category, i.condition,
	       i.estimated_value, i.min_swap_value, i.trade_interests, i.images,
	       i.city, i.latitude, i.longitude, i.status, i.views_count,
	       (SELECT COUNT(1) FROM favorites f WHERE f.item_id = i.id) as favorites_count,
	       (SELECT COUNT(1) FROM favorites f WHERE f.item_id = i.id AND f.user_id = ?) as is_favorited,
	       u.id, u.username, u.first_name, u.avatar_url, u.bio, u.city,
	       u.completed_trades_count, u.rating_average, u.rating_count, u.is_verified, u.created_at,
	       i.created_at, i.updated_at
	FROM items i
	JOIN users u ON u.id = i.user_id
	WHERE i.id = ?
	`

	row := s.db.QueryRow(query, currentUserID, id)

	item, err := scanItemWithUser(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("item not found")
		}
		return nil, err
	}

	return item, nil
}

func (s *Service) Update(id, userID string, req models.ItemUpdateRequest) (*models.Item, error) {
	current, err := s.GetByID(id, userID)
	if err != nil {
		return nil, err
	}

	if current.UserID != userID {
		return nil, errors.New("forbidden: you do not own this item")
	}

	if req.Title != nil {
		current.Title = strings.TrimSpace(*req.Title)
	}
	if req.Description != nil {
		current.Description = strings.TrimSpace(*req.Description)
	}
	if req.Category != nil {
		current.Category = *req.Category
	}
	if req.Condition != nil {
		current.Condition = *req.Condition
	}
	if req.EstimatedValue != nil {
		current.EstimatedValue = *req.EstimatedValue
	}
	if req.MinSwapValue != nil {
		current.MinSwapValue = *req.MinSwapValue
	}
	if req.TradeInterests != nil {
		current.TradeInterests = *req.TradeInterests
	}
	if req.Images != nil {
		current.Images = *req.Images
	}
	if req.City != nil {
		current.City = strings.TrimSpace(*req.City)
	}
	if req.Latitude != nil {
		current.Latitude = *req.Latitude
	}
	if req.Longitude != nil {
		current.Longitude = *req.Longitude
	}
	if req.Status != nil {
		current.Status = *req.Status
	}
	current.UpdatedAt = time.Now()

	tradeJSON, _ := json.Marshal(current.TradeInterests)
	imagesJSON, _ := json.Marshal(current.Images)

	query := `
	UPDATE items
	SET title = ?, description = ?, category = ?, condition = ?, estimated_value = ?,
	    min_swap_value = ?, trade_interests = ?, images = ?, city = ?, latitude = ?,
	    longitude = ?, status = ?, updated_at = ?
	WHERE id = ? AND user_id = ?
	`
	_, err = s.db.Exec(query,
		current.Title, current.Description, current.Category, current.Condition,
		current.EstimatedValue, current.MinSwapValue, string(tradeJSON), string(imagesJSON),
		current.City, current.Latitude, current.Longitude, current.Status, current.UpdatedAt,
		current.ID, userID,
	)
	if err != nil {
		return nil, err
	}

	return current, nil
}

func (s *Service) Delete(id, userID string) error {
	res, err := s.db.Exec("DELETE FROM items WHERE id = ? AND user_id = ?", id, userID)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return errors.New("item not found or not owned by user")
	}
	return nil
}

func (s *Service) CountActiveByUser(userID string) (int, error) {
	var count int
	query := "SELECT COUNT(1) FROM items WHERE user_id = ? AND status = ?"
	err := s.db.QueryRow(query, userID, models.ItemStatusAvailable).Scan(&count)
	return count, err
}

func (s *Service) List(filter Filter, currentUserID string) ([]models.Item, error) {
	var conditions []string
	var args []any

	args = append(args, currentUserID) // For is_favorited subquery

	if filter.Status != "" {
		conditions = append(conditions, "i.status = ?")
		args = append(args, filter.Status)
	} else if filter.UserID == "" {
		// By default on public marketplace, only show available items
		conditions = append(conditions, "i.status = 'available'")
	}

	if filter.UserID != "" {
		conditions = append(conditions, "i.user_id = ?")
		args = append(args, filter.UserID)
	}

	if filter.Category != "" {
		conditions = append(conditions, "i.category = ?")
		args = append(args, filter.Category)
	}

	if filter.Condition != "" {
		conditions = append(conditions, "i.condition = ?")
		args = append(args, filter.Condition)
	}

	if filter.MinEstimatedValue != nil {
		conditions = append(conditions, "i.estimated_value >= ?")
		args = append(args, *filter.MinEstimatedValue)
	}

	if filter.MaxEstimatedValue != nil {
		conditions = append(conditions, "i.estimated_value <= ?")
		args = append(args, *filter.MaxEstimatedValue)
	}

	if filter.Query != "" {
		q := "%" + strings.ToLower(filter.Query) + "%"
		conditions = append(conditions, "(LOWER(i.title) LIKE ? OR LOWER(i.description) LIKE ?)")
		args = append(args, q, q)
	}

	// "Ich suche ..." Reverse Trade Interest Search:
	// Find items whose trade_interests contains the query keyword!
	if filter.TradeInterestSearch != "" {
		ti := "%" + strings.ToLower(filter.TradeInterestSearch) + "%"
		conditions = append(conditions, "LOWER(i.trade_interests) LIKE ?")
		args = append(args, ti)
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	orderBy := "ORDER BY i.created_at DESC"
	switch filter.Sort {
	case "popular":
		orderBy = "ORDER BY (favorites_count * 2 + i.views_count) DESC"
	case "value_asc":
		orderBy = "ORDER BY i.estimated_value ASC"
	case "value_desc":
		orderBy = "ORDER BY i.estimated_value DESC"
	case "newest":
		orderBy = "ORDER BY i.created_at DESC"
	}

	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}

	query := fmt.Sprintf(`
	SELECT i.id, i.user_id, i.title, i.description, i.category, i.condition,
	       i.estimated_value, i.min_swap_value, i.trade_interests, i.images,
	       i.city, i.latitude, i.longitude, i.status, i.views_count,
	       (SELECT COUNT(1) FROM favorites f WHERE f.item_id = i.id) as favorites_count,
	       (SELECT COUNT(1) FROM favorites f WHERE f.item_id = i.id AND f.user_id = ?) as is_favorited,
	       u.id, u.username, u.first_name, u.avatar_url, u.bio, u.city,
	       u.completed_trades_count, u.rating_average, u.rating_count, u.is_verified, u.created_at,
	       i.created_at, i.updated_at
	FROM items i
	JOIN users u ON u.id = i.user_id
	%s
	%s
	LIMIT ? OFFSET ?
	`, whereClause, orderBy)

	args = append(args, limit, offset)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query items: %w", err)
	}
	defer rows.Close()

	var items []models.Item
	for rows.Next() {
		item, err := scanItemWithUser(rows)
		if err != nil {
			return nil, err
		}

		// Filter by distance if requested
		if filter.Latitude != nil && filter.Longitude != nil && filter.MaxDistanceKm != nil {
			dist := haversineDistance(*filter.Latitude, *filter.Longitude, item.Latitude, item.Longitude)
			if dist > *filter.MaxDistanceKm {
				continue
			}
		}

		items = append(items, *item)
	}

	return items, nil
}

func (s *Service) ToggleFavorite(userID, itemID string) (bool, error) {
	var count int
	err := s.db.QueryRow("SELECT COUNT(1) FROM favorites WHERE user_id = ? AND item_id = ?", userID, itemID).Scan(&count)
	if err != nil {
		return false, err
	}

	if count > 0 {
		// Remove favorite
		_, err := s.db.Exec("DELETE FROM favorites WHERE user_id = ? AND item_id = ?", userID, itemID)
		return false, err
	}

	// Add favorite
	_, err = s.db.Exec("INSERT INTO favorites (user_id, item_id, created_at) VALUES (?, ?, ?)", userID, itemID, time.Now())
	return true, err
}

func (s *Service) GetFavorites(userID string) ([]models.Item, error) {
	query := `
	SELECT i.id, i.user_id, i.title, i.description, i.category, i.condition,
	       i.estimated_value, i.min_swap_value, i.trade_interests, i.images,
	       i.city, i.latitude, i.longitude, i.status, i.views_count,
	       (SELECT COUNT(1) FROM favorites f WHERE f.item_id = i.id) as favorites_count,
	       1 as is_favorited,
	       u.id, u.username, u.first_name, u.avatar_url, u.bio, u.city,
	       u.completed_trades_count, u.rating_average, u.rating_count, u.is_verified, u.created_at,
	       i.created_at, i.updated_at
	FROM favorites fav
	JOIN items i ON i.id = fav.item_id
	JOIN users u ON u.id = i.user_id
	WHERE fav.user_id = ?
	ORDER BY fav.created_at DESC
	`
	rows, err := s.db.Query(query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []models.Item
	for rows.Next() {
		item, err := scanItemWithUser(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *item)
	}

	return items, nil
}

func (s *Service) GetHomeFeed(currentUserID string, userLat, userLng float64) (*models.HomeFeed, error) {
	// Categories with counts
	catRows, err := s.db.Query(`
		SELECT category, COUNT(1)
		FROM items
		WHERE status = 'available'
		GROUP BY category
	`)
	var categories []models.CategoryInfo
	if err == nil {
		defer catRows.Close()
		for catRows.Next() {
			var cat models.CategoryInfo
			if err := catRows.Scan(&cat.Name, &cat.ItemCount); err == nil {
				categories = append(categories, cat)
			}
		}
	}

	// Recent items
	recent, err := s.List(Filter{Limit: 10, Sort: "newest"}, currentUserID)
	if err != nil {
		recent = []models.Item{}
	}

	// Popular items
	popular, err := s.List(Filter{Limit: 10, Sort: "popular"}, currentUserID)
	if err != nil {
		popular = []models.Item{}
	}

	// Nearby items
	nearby := []models.Item{}
	if userLat != 0.0 || userLng != 0.0 {
		maxDist := 50.0 // 50 km default
		nearby, _ = s.List(Filter{
			Latitude:      &userLat,
			Longitude:     &userLng,
			MaxDistanceKm: &maxDist,
			Limit:         10,
		}, currentUserID)
	}

	return &models.HomeFeed{
		Categories:   categories,
		RecentItems:  recent,
		NearbyItems:  nearby,
		PopularItems: popular,
	}, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanItemWithUser(r rowScanner) (*models.Item, error) {
	var item models.Item
	var tradeInterestsRaw, imagesRaw string
	var owner models.UserPublicProfile
	var ownerFirstName, ownerAvatar, ownerBio, ownerCity sql.NullString
	var isFavoritedInt int

	err := r.Scan(
		&item.ID, &item.UserID, &item.Title, &item.Description, &item.Category, &item.Condition,
		&item.EstimatedValue, &item.MinSwapValue, &tradeInterestsRaw, &imagesRaw,
		&item.City, &item.Latitude, &item.Longitude, &item.Status, &item.ViewsCount,
		&item.FavoritesCount, &isFavoritedInt,
		&owner.ID, &owner.Username, &ownerFirstName, &ownerAvatar, &ownerBio, &ownerCity,
		&owner.CompletedTradesCount, &owner.RatingAverage, &owner.RatingCount, &owner.IsVerified, &owner.CreatedAt,
		&item.CreatedAt, &item.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	item.IsFavorited = isFavoritedInt > 0
	owner.FirstName = ownerFirstName.String
	owner.AvatarURL = ownerAvatar.String
	owner.Bio = ownerBio.String
	owner.City = ownerCity.String
	item.Owner = &owner

	_ = json.Unmarshal([]byte(tradeInterestsRaw), &item.TradeInterests)
	_ = json.Unmarshal([]byte(imagesRaw), &item.Images)

	if item.TradeInterests == nil {
		item.TradeInterests = []string{}
	}
	if item.Images == nil {
		item.Images = []string{}
	}

	return &item, nil
}

func haversineDistance(lat1, lon1, lat2, lon2 float64) float64 {
	const earthRadiusKm = 6371.0

	dLat := (lat2 - lat1) * (math.Pi / 180.0)
	dLon := (lon2 - lon1) * (math.Pi / 180.0)

	rLat1 := lat1 * (math.Pi / 180.0)
	rLat2 := lat2 * (math.Pi / 180.0)

	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Sin(dLon/2)*math.Sin(dLon/2)*math.Cos(rLat1)*math.Cos(rLat2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

	return earthRadiusKm * c
}
