// Package users covers profiles, blocking, reporting, in-app notifications
// and account deletion.
package users

import (
	"context"
	"errors"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/yasinatby/swappy/backend/internal/apperr"
	"github.com/yasinatby/swappy/backend/internal/ids"
	"github.com/yasinatby/swappy/backend/internal/swaps"
)

var (
	ErrProfileNotFound = apperr.New(apperr.NotFound, "profile_not_found", "profile not found")
	ErrNameTaken       = apperr.New(apperr.Conflict, "display_name_taken", "this name is already taken")
	ErrCannotBlockSelf = apperr.New(apperr.Invalid, "invalid_input", "you cannot block yourself")
)

var ReportReasons = []string{"spam", "prohibited_item", "offensive", "scam", "other"}

var (
	displayNamePattern = regexp.MustCompile(`^[\p{L}\p{N}_.-]{3,30}$`)
	postalCodePattern  = regexp.MustCompile(`^[0-9]{5}$`)
)

// Profile never contains an address or coordinates (invariant 8).
type Profile struct {
	ID          string    `json:"id"`
	DisplayName string    `json:"display_name"`
	PostalCode  string    `json:"postal_code,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	ActiveItems int       `json:"active_items"`
	Trades      int       `json:"completed_trades"`
	DistanceKm  *int      `json:"distance_km,omitempty"`
}

type Notification struct {
	ID        string     `json:"id"`
	Kind      string     `json:"kind"`
	OfferID   *string    `json:"offer_id"`
	CreatedAt time.Time  `json:"created_at"`
	ReadAt    *time.Time `json:"read_at"`
}

type Service struct {
	pool  *pgxpool.Pool
	swaps *swaps.Service
}

func NewService(pool *pgxpool.Pool, swapService *swaps.Service) *Service {
	return &Service{pool: pool, swaps: swapService}
}

// IsActive reports whether the user has a profile that is not deleted.
func (s *Service) IsActive(ctx context.Context, userID string) (bool, error) {
	var ok bool
	err := s.pool.QueryRow(ctx, `
		select exists(select 1 from profiles where id = $1 and deleted_at is null)`, userID).Scan(&ok)
	return ok, err
}

const selectProfile = `
	select p.id, p.display_name, p.postal_code, p.created_at,
	       (select count(*) from items where owner_id = p.id and status = 'active'),
	       (select count(*) from trade_offers where status = 'completed' and (initiator_id = p.id or responder_id = p.id)),
	       distance_km(me.geo_lat, me.geo_lon, p.geo_lat, p.geo_lon)
	from profiles p
	left join profiles me on me.id = $1
`

func scanProfile(row pgx.CollectableRow) (Profile, error) {
	var p Profile
	err := row.Scan(&p.ID, &p.DisplayName, &p.PostalCode, &p.CreatedAt, &p.ActiveItems, &p.Trades, &p.DistanceKm)
	return p, err
}

func (s *Service) Me(ctx context.Context, userID string) (*Profile, error) {
	rows, err := s.pool.Query(ctx, selectProfile+`where p.id = $1 and p.deleted_at is null`, userID)
	if err != nil {
		return nil, err
	}
	p, err := pgx.CollectExactlyOneRow(rows, scanProfile)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrProfileNotFound
	}
	p.DistanceKm = nil
	return &p, err
}

// Public returns another user's profile. Blocks hide profiles both ways
// (invariant 6); the postal code is not exposed.
func (s *Service) Public(ctx context.Context, viewerID, profileID string) (*Profile, error) {
	if !ids.IsUUID(profileID) {
		return nil, ErrProfileNotFound
	}
	rows, err := s.pool.Query(ctx, selectProfile+`
		where p.id = $2 and p.deleted_at is null
		  and not exists (select 1 from blocks
		                  where (blocker_id = p.id and blocked_id = $1) or (blocker_id = $1 and blocked_id = p.id))`,
		viewerID, profileID)
	if err != nil {
		return nil, err
	}
	p, err := pgx.CollectExactlyOneRow(rows, scanProfile)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrProfileNotFound
	}
	if err != nil {
		return nil, err
	}
	p.PostalCode = ""
	return &p, nil
}

type ProfileInput struct {
	DisplayName string `json:"display_name"`
	PostalCode  string `json:"postal_code"`
}

// UpdateMe changes name and postal code. A new postal code clears the stored
// centroid until a postal-code lookup exists; distances then stay hidden.
func (s *Service) UpdateMe(ctx context.Context, userID string, in ProfileInput) (*Profile, error) {
	in.DisplayName = strings.TrimSpace(in.DisplayName)
	in.PostalCode = strings.TrimSpace(in.PostalCode)
	if !displayNamePattern.MatchString(in.DisplayName) {
		return nil, apperr.New(apperr.Invalid, "invalid_input", "display name must be 3–30 letters, digits, _ . or -")
	}
	if !postalCodePattern.MatchString(in.PostalCode) {
		return nil, apperr.New(apperr.Invalid, "invalid_input", "postal code must have 5 digits")
	}
	_, err := s.pool.Exec(ctx, `
		update profiles set display_name = $2,
		       geo_lat = case when postal_code = $3 then geo_lat end,
		       geo_lon = case when postal_code = $3 then geo_lon end,
		       postal_code = $3
		where id = $1 and deleted_at is null`, userID, in.DisplayName, in.PostalCode)
	if isUniqueViolation(err) {
		return nil, ErrNameTaken
	}
	if err != nil {
		return nil, err
	}
	return s.Me(ctx, userID)
}

// Block hides both users from each other and cancels all open offers between
// them (invariant 6).
func (s *Service) Block(ctx context.Context, blockerID, blockedID string) error {
	if blockerID == blockedID {
		return ErrCannotBlockSelf
	}
	if !ids.IsUUID(blockedID) {
		return ErrProfileNotFound
	}
	tag, err := s.pool.Exec(ctx, `
		insert into blocks (blocker_id, blocked_id)
		select $1, id from profiles where id = $2
		on conflict do nothing`, blockerID, blockedID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		var exists bool
		if err := s.pool.QueryRow(ctx, `select exists(select 1 from profiles where id = $1)`, blockedID).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			return ErrProfileNotFound
		}
	}
	return s.swaps.CancelOffersBetween(ctx, blockerID, blockedID)
}

func (s *Service) Unblock(ctx context.Context, blockerID, blockedID string) error {
	if !ids.IsUUID(blockedID) {
		return ErrProfileNotFound
	}
	_, err := s.pool.Exec(ctx, `delete from blocks where blocker_id = $1 and blocked_id = $2`, blockerID, blockedID)
	return err
}

type BlockedUser struct {
	ID          string    `json:"id"`
	DisplayName string    `json:"display_name"`
	BlockedAt   time.Time `json:"blocked_at"`
}

func (s *Service) Blocked(ctx context.Context, blockerID string) ([]BlockedUser, error) {
	rows, err := s.pool.Query(ctx, `
		select p.id, p.display_name, b.created_at
		from blocks b join profiles p on p.id = b.blocked_id
		where b.blocker_id = $1 order by b.created_at desc`, blockerID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByPos[BlockedUser])
}

type ReportInput struct {
	SubjectType string  `json:"subject_type"`
	SubjectID   string  `json:"subject_id"`
	Reason      string  `json:"reason"`
	Note        *string `json:"note"`
}

// Report files a moderation report. Reports are reviewed within 24 hours.
func (s *Service) Report(ctx context.Context, reporterID string, in ReportInput) error {
	if in.SubjectType != "item" && in.SubjectType != "profile" {
		return apperr.New(apperr.Invalid, "invalid_input", "subject_type must be item or profile")
	}
	if !ids.IsUUID(in.SubjectID) {
		return apperr.New(apperr.Invalid, "invalid_input", "subject_id must be a uuid")
	}
	if !slices.Contains(ReportReasons, in.Reason) {
		return apperr.New(apperr.Invalid, "invalid_input", "unknown reason")
	}
	if in.Note != nil && utf8.RuneCountInString(*in.Note) > 1000 {
		return apperr.New(apperr.Invalid, "invalid_input", "note must be at most 1000 characters")
	}
	table := "items"
	if in.SubjectType == "profile" {
		table = "profiles"
	}
	tag, err := s.pool.Exec(ctx, `
		insert into reports (reporter_id, subject_type, subject_id, reason, note)
		select $1, $2, id, $4::report_reason, $5 from `+table+` where id = $3`,
		reporterID, in.SubjectType, in.SubjectID, in.Reason, in.Note)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return apperr.New(apperr.NotFound, "subject_not_found", "the reported content does not exist")
	}
	return nil
}

func (s *Service) Notifications(ctx context.Context, userID string) ([]Notification, error) {
	rows, err := s.pool.Query(ctx, `
		select id, kind, offer_id, created_at, read_at from notifications
		where user_id = $1 order by created_at desc limit 100`, userID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByPos[Notification])
}

func (s *Service) MarkNotificationsRead(ctx context.Context, userID string) error {
	_, err := s.pool.Exec(ctx, `update notifications set read_at = now() where user_id = $1 and read_at is null`, userID)
	return err
}

// DeleteAccount is a soft delete (invariant 7): the profile is anonymized and
// marked deleted first so nothing new can start, then open offers are
// cancelled and remaining items withdrawn. Completed trades stay in history
// under the anonymized profile.
func (s *Service) DeleteAccount(ctx context.Context, userID string) error {
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `
			update profiles set deleted_at = now(), display_name = 'geloescht-' || left(id::text, 8),
			       avatar_path = null, postal_code = '', geo_lat = null, geo_lon = null
			where id = $1 and deleted_at is null`, userID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			update users set deleted_at = now(), email = null, password_hash = null, apple_sub = null
			where id = $1`, userID); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `delete from device_tokens where user_id = $1`, userID)
		return err
	})
	if err != nil {
		return err
	}
	if err := s.swaps.CancelAllOffersOf(ctx, userID); err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `
		update items set status = 'deleted' where owner_id = $1 and status in ('draft', 'active')`, userID)
	return err
}

// DevLogin returns the profile with this display name, creating a user and
// profile if needed. Only reachable when SWAPPY_DEV_LOGIN is enabled.
func (s *Service) DevLogin(ctx context.Context, displayName string) (*Profile, error) {
	displayName = strings.TrimSpace(displayName)
	if !displayNamePattern.MatchString(displayName) {
		return nil, apperr.New(apperr.Invalid, "invalid_input", "display name must be 3–30 letters, digits, _ . or -")
	}
	var id string
	err := s.pool.QueryRow(ctx, `select id from profiles where display_name = $1 and deleted_at is null`, displayName).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		err = s.pool.QueryRow(ctx, `
			with u as (insert into users (password_hash) values ('!dev-login') returning id)
			insert into profiles (id, display_name, postal_code, geo_lat, geo_lon, terms_accepted_at)
			select id, $1, '10115', 52.53, 13.38, now() from u returning id`, displayName).Scan(&id)
	}
	if err != nil {
		return nil, err
	}
	return s.Me(ctx, id)
}

// DevProfiles lists active profiles for the dev login picker.
func (s *Service) DevProfiles(ctx context.Context) ([]Profile, error) {
	rows, err := s.pool.Query(ctx, selectProfile+`where p.deleted_at is null order by p.display_name limit 50`, "00000000-0000-0000-0000-000000000000")
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, scanProfile)
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
