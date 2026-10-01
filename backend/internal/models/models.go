package models

import "time"

// User Roles
const (
	RoleUser  = "user"
	RoleAdmin = "admin"
)

// Item Categories
var ValidCategories = []string{
	"Gaming",
	"Elektronik",
	"Fahrzeuge",
	"Kleidung",
	"Haushalt",
	"Hobby",
	"Bücher",
	"Uhren",
	"Sonstiges",
}

// Item Conditions
var ValidConditions = []string{
	"Neu",
	"Wie neu",
	"Sehr gut",
	"Gut",
	"Akzeptabel",
}

// Item Status
const (
	ItemStatusAvailable   = "available"   // 🟢
	ItemStatusRequested   = "requested"   // 🟡
	ItemStatusTraded      = "traded"      // 🔴
	ItemStatusDeactivated = "deactivated" // ⚪
)

// Swap Status
const (
	SwapStatusPending   = "pending"
	SwapStatusCountered = "countered"
	SwapStatusAgreed    = "agreed"
	SwapStatusCompleted = "completed"
	SwapStatusRejected  = "rejected"
	SwapStatusCancelled = "cancelled"
)

// Handover Methods
const (
	HandoverPersonal = "personal" // persönliche Übergabe
	HandoverShipping = "shipping" // Versand
)

// Notification Types
const (
	NotifSwapProposal  = "swap_proposal"
	NotifSwapCounter   = "swap_counter"
	NotifSwapAccepted  = "swap_accepted"
	NotifSwapRejected  = "swap_rejected"
	NotifSwapCompleted = "swap_completed"
	NotifNewMessage    = "new_message"
	NotifReviewReceived = "review_received"
	NotifItemFavorited = "item_favorited"
)

// User represents a user account
type User struct {
	ID                   string    `json:"id"`
	Username             string    `json:"username"`
	Email                string    `json:"email"`
	PasswordHash         string    `json:"-"`
	FirstName            string    `json:"first_name,omitempty"`
	AvatarURL            string    `json:"avatar_url,omitempty"`
	Bio                  string    `json:"bio,omitempty"`
	City                 string    `json:"city,omitempty"`
	PostalCode           string    `json:"postal_code,omitempty"`
	Latitude             float64   `json:"latitude,omitempty"`
	Longitude            float64   `json:"longitude,omitempty"`
	IsVerified           bool      `json:"is_verified"`
	IsBanned             bool      `json:"is_banned"`
	Role                 string    `json:"role"`
	CompletedTradesCount int       `json:"completed_trades_count"`
	RatingAverage        float64   `json:"rating_average"`
	RatingCount          int       `json:"rating_count"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
}

// UserPublicProfile represents the public view of a user
type UserPublicProfile struct {
	ID                   string    `json:"id"`
	Username             string    `json:"username"`
	FirstName            string    `json:"first_name,omitempty"`
	AvatarURL            string    `json:"avatar_url,omitempty"`
	Bio                  string    `json:"bio,omitempty"`
	City                 string    `json:"city,omitempty"`
	CompletedTradesCount int       `json:"completed_trades_count"`
	RatingAverage        float64   `json:"rating_average"`
	RatingCount          int       `json:"rating_count"`
	IsVerified           bool      `json:"is_verified"`
	CreatedAt            time.Time `json:"created_at"`
	ActiveItems          []Item    `json:"active_items,omitempty"`
	TradedItems          []Item    `json:"traded_items,omitempty"`
}

func (u *User) ToPublic() UserPublicProfile {
	return UserPublicProfile{
		ID:                   u.ID,
		Username:             u.Username,
		FirstName:            u.FirstName,
		AvatarURL:            u.AvatarURL,
		Bio:                  u.Bio,
		City:                 u.City,
		CompletedTradesCount: u.CompletedTradesCount,
		RatingAverage:        u.RatingAverage,
		RatingCount:          u.RatingCount,
		IsVerified:           u.IsVerified,
		CreatedAt:            u.CreatedAt,
	}
}

// Item represents a listed object for barter
type Item struct {
	ID             string             `json:"id"`
	UserID         string             `json:"user_id"`
	Title          string             `json:"title"`
	Description    string             `json:"description"`
	Category       string             `json:"category"`
	Condition      string             `json:"condition"`
	EstimatedValue float64            `json:"estimated_value"`
	MinSwapValue   float64            `json:"min_swap_value,omitempty"`
	TradeInterests []string           `json:"trade_interests"`
	Images         []string           `json:"images"`
	City           string             `json:"city,omitempty"`
	Latitude       float64            `json:"latitude,omitempty"`
	Longitude      float64            `json:"longitude,omitempty"`
	Status         string             `json:"status"` // available, requested, traded, deactivated
	ViewsCount     int                `json:"views_count"`
	FavoritesCount int                `json:"favorites_count"`
	IsFavorited    bool               `json:"is_favorited,omitempty"`
	Owner          *UserPublicProfile `json:"owner,omitempty"`
	CreatedAt      time.Time          `json:"created_at"`
	UpdatedAt      time.Time          `json:"updated_at"`
}

// Swap represents a barter proposal and negotiation
type Swap struct {
	ID                 string             `json:"id"`
	InitiatorID        string             `json:"initiator_id"`
	RecipientID        string             `json:"recipient_id"`
	TargetItemID       string             `json:"target_item_id"`
	TargetItem         *Item              `json:"target_item,omitempty"`
	OfferedItems       []Item             `json:"offered_items,omitempty"`
	Status             string             `json:"status"` // pending, countered, agreed, completed, rejected, cancelled
	CashAdjustment     float64            `json:"cash_adjustment"` // positive: initiator offers cash; negative: recipient asked to add cash
	Message            string             `json:"message,omitempty"`
	HandoverMethod     string             `json:"handover_method"` // personal, shipping
	InitiatorConfirmed bool               `json:"initiator_confirmed"`
	RecipientConfirmed bool               `json:"recipient_confirmed"`
	Initiator          *UserPublicProfile `json:"initiator,omitempty"`
	Recipient          *UserPublicProfile `json:"recipient,omitempty"`
	History            []SwapHistory      `json:"history,omitempty"`
	CreatedAt          time.Time          `json:"created_at"`
	UpdatedAt          time.Time          `json:"updated_at"`
}

// SwapHistory tracks negotiation rounds / counter-offers
type SwapHistory struct {
	ID             string    `json:"id"`
	SwapID         string    `json:"swap_id"`
	ActorID        string    `json:"actor_id"`
	ActorUsername  string    `json:"actor_username,omitempty"`
	Action         string    `json:"action"` // proposed, countered, accepted, rejected, completed, cancelled
	Message        string    `json:"message,omitempty"`
	CashAdjustment float64   `json:"cash_adjustment"`
	OfferedItemIDs []string  `json:"offered_item_ids"`
	CreatedAt      time.Time `json:"created_at"`
}

// Message represents a chat message within a swap context
type Message struct {
	ID        string             `json:"id"`
	SwapID    string             `json:"swap_id"`
	SenderID  string             `json:"sender_id"`
	Sender    *UserPublicProfile `json:"sender,omitempty"`
	Text      string             `json:"text,omitempty"`
	ImageURL  string             `json:"image_url,omitempty"`
	IsSystem  bool               `json:"is_system"`
	CreatedAt time.Time          `json:"created_at"`
}

// Review represents feedback after a completed trade
type Review struct {
	ID            string             `json:"id"`
	SwapID        string             `json:"swap_id"`
	ReviewerID    string             `json:"reviewer_id"`
	TargetID      string             `json:"target_id"`
	Rating        int                `json:"rating"` // 1-5
	Reliability   int                `json:"reliability"`
	Communication int                `json:"communication"`
	ItemCondition int                `json:"item_condition"`
	Adherence     int                `json:"adherence"`
	Comment       string             `json:"comment"`
	Reviewer      *UserPublicProfile `json:"reviewer,omitempty"`
	CreatedAt     time.Time          `json:"created_at"`
}

// Notification represents a user alert
type Notification struct {
	ID         string    `json:"id"`
	UserID     string    `json:"user_id"`
	Type       string    `json:"type"`
	Title      string    `json:"title"`
	Body       string    `json:"body"`
	EntityType string    `json:"entity_type,omitempty"`
	EntityID   string    `json:"entity_id,omitempty"`
	IsRead     bool      `json:"is_read"`
	CreatedAt  time.Time `json:"created_at"`
}

// Report represents a flagged user or item
type Report struct {
	ID           string    `json:"id"`
	ReporterID   string    `json:"reporter_id"`
	TargetUserID string    `json:"target_user_id,omitempty"`
	TargetItemID string    `json:"target_item_id,omitempty"`
	Reason       string    `json:"reason"`
	Description  string    `json:"description,omitempty"`
	Status       string    `json:"status"` // open, resolved, dismissed
	CreatedAt    time.Time `json:"created_at"`
}

// SwapMatch represents a discovered mutual barter match
type SwapMatch struct {
	MyItem       Item              `json:"my_item"`
	TheirItem    Item              `json:"their_item"`
	OtherUser    UserPublicProfile `json:"other_user"`
	MatchScore   int               `json:"match_score"` // 1-100
	MatchReason  string            `json:"match_reason"`
	EstimatedDiff float64          `json:"estimated_diff"` // value difference
}

// CategoryInfo holds category name and count
type CategoryInfo struct {
	Name      string `json:"name"`
	ItemCount int    `json:"item_count"`
}

// HomeFeed aggregates feed sections for the start page
type HomeFeed struct {
	Categories   []CategoryInfo `json:"categories"`
	RecentItems  []Item         `json:"recent_items"`
	NearbyItems  []Item         `json:"nearby_items"`
	PopularItems []Item         `json:"popular_items"`
}

// AdminStats provides platform overview metrics
type AdminStats struct {
	TotalUsers           int     `json:"total_users"`
	ActiveItems          int     `json:"active_items"`
	CompletedTrades      int     `json:"completed_trades"`
	OpenReports          int     `json:"open_reports"`
	BannedUsers          int     `json:"banned_users"`
	TotalEstimatedVolume float64 `json:"total_estimated_volume"`
}

// API Request/Response Types

type RegisterRequest struct {
	Username  string  `json:"username"`
	Email     string  `json:"email"`
	Password  string  `json:"password"`
	FirstName string  `json:"first_name,omitempty"`
	City      string  `json:"city,omitempty"`
	Latitude  float64 `json:"latitude,omitempty"`
	Longitude float64 `json:"longitude,omitempty"`
}

type LoginRequest struct {
	UsernameOrEmail string `json:"username_or_email"`
	Password        string `json:"password"`
}

type AuthResponse struct {
	Token string `json:"token"`
	User  User   `json:"user"`
}

type UpdateProfileRequest struct {
	FirstName  *string  `json:"first_name,omitempty"`
	Bio        *string  `json:"bio,omitempty"`
	AvatarURL  *string  `json:"avatar_url,omitempty"`
	City       *string  `json:"city,omitempty"`
	PostalCode *string  `json:"postal_code,omitempty"`
	Latitude   *float64 `json:"latitude,omitempty"`
	Longitude  *float64 `json:"longitude,omitempty"`
}

type ItemCreateRequest struct {
	Title          string   `json:"title"`
	Description    string   `json:"description"`
	Category       string   `json:"category"`
	Condition      string   `json:"condition"`
	EstimatedValue float64  `json:"estimated_value"`
	MinSwapValue   float64  `json:"min_swap_value,omitempty"`
	TradeInterests []string `json:"trade_interests"`
	Images         []string `json:"images"`
	City           string   `json:"city,omitempty"`
	Latitude       float64  `json:"latitude,omitempty"`
	Longitude      float64  `json:"longitude,omitempty"`
}

type ItemUpdateRequest struct {
	Title          *string   `json:"title,omitempty"`
	Description    *string   `json:"description,omitempty"`
	Category       *string   `json:"category,omitempty"`
	Condition      *string   `json:"condition,omitempty"`
	EstimatedValue *float64  `json:"estimated_value,omitempty"`
	MinSwapValue   *float64  `json:"min_swap_value,omitempty"`
	TradeInterests *[]string `json:"trade_interests,omitempty"`
	Images         *[]string `json:"images,omitempty"`
	City           *string   `json:"city,omitempty"`
	Latitude       *float64  `json:"latitude,omitempty"`
	Longitude      *float64  `json:"longitude,omitempty"`
	Status         *string   `json:"status,omitempty"`
}

type SwapProposalRequest struct {
	TargetItemID   string   `json:"target_item_id"`
	OfferedItemIDs []string `json:"offered_item_ids"`
	CashAdjustment float64  `json:"cash_adjustment"` // e.g. +50 means initiator pays 50
	Message        string   `json:"message"`
	HandoverMethod string   `json:"handover_method,omitempty"` // personal, shipping
}

type SwapCounterRequest struct {
	OfferedItemIDs []string `json:"offered_item_ids"`
	CashAdjustment float64  `json:"cash_adjustment"`
	Message        string   `json:"message"`
}

type SendMessageRequest struct {
	Text     string `json:"text,omitempty"`
	ImageURL string `json:"image_url,omitempty"`
}

type CreateReviewRequest struct {
	Rating        int    `json:"rating"` // 1-5
	Reliability   int    `json:"reliability"`
	Communication int    `json:"communication"`
	ItemCondition int    `json:"item_condition"`
	Adherence     int    `json:"adherence"`
	Comment       string `json:"comment"`
}

type CreateReportRequest struct {
	TargetUserID string `json:"target_user_id,omitempty"`
	TargetItemID string `json:"target_item_id,omitempty"`
	Reason       string `json:"reason"`
	Description  string `json:"description"`
}
