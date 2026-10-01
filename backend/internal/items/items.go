// Package items manages a user's items, their images and the public feed.
// Deletion lives in swaps because it cancels open offers.
package items

import (
	"context"
	"errors"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/yasinatby/swappy/backend/internal/apperr"
	"github.com/yasinatby/swappy/backend/internal/ids"
)

const MaxImages = 6

// Categories is the fixed category list; the app ships the same keys.
var Categories = []string{
	"elektronik", "haushalt", "moebel", "kleidung", "buecher", "musik",
	"sport", "spiele", "kinder", "garten", "werkzeug", "sonstiges",
}

var Conditions = []string{"new", "good", "used"}

var (
	ErrNotFound      = apperr.New(apperr.NotFound, "item_not_found", "item not found")
	ErrLocked        = apperr.New(apperr.Conflict, "item_locked", "the item can only be changed while it is a draft or active")
	ErrTooManyImages = apperr.New(apperr.Conflict, "too_many_images", "an item can have at most 6 images")
	ErrImageNotFound = apperr.New(apperr.NotFound, "image_not_found", "image not found")
)

type Image struct {
	ID       string `json:"id"`
	Path     string `json:"path"`
	Position int    `json:"position"`
}

type Item struct {
	ID          string    `json:"id"`
	OwnerID     string    `json:"owner_id"`
	OwnerName   string    `json:"owner_name"`
	Title       string    `json:"title"`
	Description *string   `json:"description"`
	Category    string    `json:"category"`
	Condition   string    `json:"condition"`
	WantedText  *string   `json:"wanted_text"`
	Status      string    `json:"status"`
	CoverPath   *string   `json:"cover_path"`
	DistanceKm  *int      `json:"distance_km"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	Images      []Image   `json:"images,omitempty"`
}

type Input struct {
	Title       string  `json:"title"`
	Description *string `json:"description"`
	Category    string  `json:"category"`
	Condition   string  `json:"condition"`
	WantedText  *string `json:"wanted_text"`
}

func (in *Input) normalize() error {
	in.Title = strings.TrimSpace(in.Title)
	in.Description = trimOptional(in.Description)
	in.WantedText = trimOptional(in.WantedText)
	switch {
	case in.Title == "" || utf8.RuneCountInString(in.Title) > 80:
		return invalid("title must be 1–80 characters")
	case in.Description != nil && utf8.RuneCountInString(*in.Description) > 2000:
		return invalid("description must be at most 2000 characters")
	case in.WantedText != nil && utf8.RuneCountInString(*in.WantedText) > 300:
		return invalid("wanted_text must be at most 300 characters")
	case !slices.Contains(Categories, in.Category):
		return invalid("unknown category")
	case !slices.Contains(Conditions, in.Condition):
		return invalid("condition must be new, good or used")
	}
	return nil
}

type Service struct {
	pool *pgxpool.Pool
}

func NewService(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool}
}

// selectItem is shared by every read. $1 is always the viewer.
const selectItem = `
	select i.id, i.owner_id, p.display_name, i.title, i.description, i.category, i.condition::text,
	       i.wanted_text, i.status::text,
	       (select storage_path from item_images im where im.item_id = i.id order by position limit 1),
	       distance_km(me.geo_lat, me.geo_lon, p.geo_lat, p.geo_lon),
	       i.created_at, i.updated_at
	from items i
	join profiles p on p.id = i.owner_id
	left join profiles me on me.id = $1
`

// notBlocked excludes items whose owner blocked the viewer or vice versa.
const notBlocked = `
	not exists (select 1 from blocks b
	            where (b.blocker_id = $1 and b.blocked_id = i.owner_id)
	               or (b.blocker_id = i.owner_id and b.blocked_id = $1))
`

type FeedQuery struct {
	Search   string
	Category string
	OwnerID  string
	Offset   int
}

// Feed lists active items of other users, newest first.
func (s *Service) Feed(ctx context.Context, viewerID string, q FeedQuery) ([]Item, error) {
	pattern := ""
	if q.Search = strings.TrimSpace(q.Search); q.Search != "" {
		pattern = "%" + strings.NewReplacer(`\`, `\\`, "%", `\%`, "_", `\_`).Replace(q.Search) + "%"
	}
	rows, err := s.pool.Query(ctx, selectItem+`
		where i.status = 'active' and i.owner_id <> $1 and p.deleted_at is null and `+notBlocked+`
		  and ($2 = '' or i.title ilike $2 or i.description ilike $2 or i.wanted_text ilike $2)
		  and ($3 = '' or i.category = $3)
		  and ($4 = '' or i.owner_id::text = $4)
		order by i.created_at desc, i.id
		limit 50 offset $5`, viewerID, pattern, q.Category, q.OwnerID, max(q.Offset, 0))
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, scanItem)
}

// Mine lists the viewer's own items (everything except deleted).
func (s *Service) Mine(ctx context.Context, viewerID string) ([]Item, error) {
	rows, err := s.pool.Query(ctx, selectItem+`
		where i.owner_id = $1 and i.status <> 'deleted'
		order by i.created_at desc`, viewerID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, scanItem)
}

// Get returns an item if the viewer may see it: their own, any active item
// from a non-blocked user, or an item in one of the viewer's offers.
func (s *Service) Get(ctx context.Context, viewerID, itemID string) (*Item, error) {
	if !ids.IsUUID(itemID) {
		return nil, ErrNotFound
	}
	rows, err := s.pool.Query(ctx, selectItem+`
		where i.id = $2 and i.status <> 'deleted' and (
		      i.owner_id = $1
		   or (i.status = 'active' and p.deleted_at is null and `+notBlocked+`)
		   or exists (select 1 from trade_offer_items toi join trade_offers o on o.id = toi.offer_id
		              where toi.item_id = i.id and (o.initiator_id = $1 or o.responder_id = $1)))`,
		viewerID, itemID)
	if err != nil {
		return nil, err
	}
	item, err := pgx.CollectExactlyOneRow(rows, scanItem)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	imgRows, err := s.pool.Query(ctx, `
		select id, storage_path, position from item_images where item_id = $1 order by position`, itemID)
	if err != nil {
		return nil, err
	}
	item.Images, err = pgx.CollectRows(imgRows, pgx.RowToStructByPos[Image])
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (s *Service) Create(ctx context.Context, ownerID string, in Input, publish bool) (*Item, error) {
	if err := in.normalize(); err != nil {
		return nil, err
	}
	status := "draft"
	if publish {
		status = "active"
	}
	var id string
	if err := s.pool.QueryRow(ctx, `
		insert into items (owner_id, title, description, category, condition, wanted_text, status)
		values ($1, $2, $3, $4, $5, $6, $7) returning id`,
		ownerID, in.Title, in.Description, in.Category, in.Condition, in.WantedText, status).Scan(&id); err != nil {
		return nil, err
	}
	return s.Get(ctx, ownerID, id)
}

func (s *Service) Update(ctx context.Context, ownerID, itemID string, in Input) (*Item, error) {
	if err := in.normalize(); err != nil {
		return nil, err
	}
	err := s.withOwnEditableItem(ctx, ownerID, itemID, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `
			update items set title = $2, description = $3, category = $4, condition = $5, wanted_text = $6
			where id = $1`, itemID, in.Title, in.Description, in.Category, in.Condition, in.WantedText)
		return err
	})
	if err != nil {
		return nil, err
	}
	return s.Get(ctx, ownerID, itemID)
}

// Publish moves a draft to active. Publishing an active item is a no-op.
func (s *Service) Publish(ctx context.Context, ownerID, itemID string) (*Item, error) {
	err := s.withOwnEditableItem(ctx, ownerID, itemID, func(tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `update items set status = 'active' where id = $1 and status = 'draft'`, itemID)
		return err
	})
	if err != nil {
		return nil, err
	}
	return s.Get(ctx, ownerID, itemID)
}

// AddImage records an already stored image file as the item's last image.
func (s *Service) AddImage(ctx context.Context, ownerID, itemID, storagePath string) (*Image, error) {
	var img Image
	err := s.withOwnEditableItem(ctx, ownerID, itemID, func(tx pgx.Tx) error {
		var count, next int
		if err := tx.QueryRow(ctx, `
			select count(*), coalesce(max(position) + 1, 0) from item_images where item_id = $1`,
			itemID).Scan(&count, &next); err != nil {
			return err
		}
		if count >= MaxImages {
			return ErrTooManyImages
		}
		return tx.QueryRow(ctx, `
			insert into item_images (item_id, storage_path, position) values ($1, $2, $3)
			returning id, storage_path, position`, itemID, storagePath, next).Scan(&img.ID, &img.Path, &img.Position)
	})
	if err != nil {
		return nil, err
	}
	return &img, nil
}

// DeleteImage removes the image row and returns its storage path so the
// caller can delete the file.
func (s *Service) DeleteImage(ctx context.Context, ownerID, itemID, imageID string) (string, error) {
	if !ids.IsUUID(imageID) {
		return "", ErrImageNotFound
	}
	var path string
	err := s.withOwnEditableItem(ctx, ownerID, itemID, func(tx pgx.Tx) error {
		err := tx.QueryRow(ctx, `
			delete from item_images where id = $1 and item_id = $2 returning storage_path`,
			imageID, itemID).Scan(&path)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrImageNotFound
		}
		return err
	})
	return path, err
}

// withOwnEditableItem locks the item and runs fn only if the caller owns it
// and it is still a draft or active (invariant 5).
func (s *Service) withOwnEditableItem(ctx context.Context, ownerID, itemID string, fn func(pgx.Tx) error) error {
	if !ids.IsUUID(itemID) {
		return ErrNotFound
	}
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		var owner, status string
		err := tx.QueryRow(ctx, `select owner_id, status from items where id = $1 for update`, itemID).Scan(&owner, &status)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if owner != ownerID || status == "deleted" {
			return ErrNotFound
		}
		if status != "draft" && status != "active" {
			return ErrLocked
		}
		return fn(tx)
	})
}

func scanItem(row pgx.CollectableRow) (Item, error) {
	var it Item
	err := row.Scan(&it.ID, &it.OwnerID, &it.OwnerName, &it.Title, &it.Description, &it.Category, &it.Condition,
		&it.WantedText, &it.Status, &it.CoverPath, &it.DistanceKm, &it.CreatedAt, &it.UpdatedAt)
	return it, err
}

func invalid(message string) error {
	return apperr.New(apperr.Invalid, "invalid_input", message)
}

func trimOptional(s *string) *string {
	if s == nil {
		return nil
	}
	t := strings.TrimSpace(*s)
	if t == "" {
		return nil
	}
	return &t
}
