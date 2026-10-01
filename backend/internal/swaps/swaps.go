// Package swaps implements the trade offer lifecycle:
//
//	pending ─┬→ accepted ─┬→ completed
//	         │            └→ cancelled
//	         ├→ declined
//	         ├→ cancelled
//	         └→ expired
//
// Locking rule for every operation that changes an offer: lock the offer's
// items first (FOR UPDATE, ordered by id), then the offer row. A single global
// order is what keeps two concurrent acceptances of overlapping offers from
// deadlocking — the loser blocks on the first shared item and then sees it
// reserved.
package swaps

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/yasinatby/swappy/backend/internal/apperr"
	"github.com/yasinatby/swappy/backend/internal/ids"
)

const MaxItemsPerSide = 10

var (
	ErrInvalidInput     = apperr.New(apperr.Invalid, "invalid_input", "the request is invalid")
	ErrNoActiveItem     = apperr.New(apperr.Conflict, "no_active_item", "you need at least one active item to make an offer")
	ErrItemUnavailable  = apperr.New(apperr.Conflict, "item_unavailable", "one or more items are no longer available")
	ErrOfferNotFound    = apperr.New(apperr.NotFound, "offer_not_found", "offer not found")
	ErrNotAllowed       = apperr.New(apperr.Forbidden, "not_allowed", "you are not allowed to do this")
	ErrOfferNotPending  = apperr.New(apperr.Conflict, "offer_not_pending", "the offer is no longer open")
	ErrOfferNotAccepted = apperr.New(apperr.Conflict, "offer_not_accepted", "the offer has not been accepted")
	ErrOfferExpired     = apperr.New(apperr.Conflict, "offer_expired", "the offer has expired")
	ErrBlocked          = apperr.New(apperr.Forbidden, "blocked", "you cannot trade with this user")
	ErrAlreadyConfirmed = apperr.New(apperr.Conflict, "already_confirmed", "you have already confirmed this trade")
	ErrItemNotFound     = apperr.New(apperr.NotFound, "item_not_found", "item not found")
	ErrItemLocked       = apperr.New(apperr.Conflict, "item_locked", "the item is part of an accepted trade and cannot be changed")
)

type Status string

const (
	StatusPending   Status = "pending"
	StatusAccepted  Status = "accepted"
	StatusDeclined  Status = "declined"
	StatusCancelled Status = "cancelled"
	StatusExpired   Status = "expired"
	StatusCompleted Status = "completed"
)

// Notification kinds written to the notifications outbox.
const (
	NotifyOfferReceived  = "offer_received"
	NotifyOfferAccepted  = "offer_accepted"
	NotifyOfferDeclined  = "offer_declined"
	NotifyOfferCancelled = "offer_cancelled"
	NotifyOfferExpired   = "offer_expired"
	NotifyTradeConfirmed = "trade_confirmed"
	NotifyTradeCompleted = "trade_completed"
)

type Offer struct {
	ID                   string     `json:"id"`
	InitiatorID          string     `json:"initiator_id"`
	ResponderID          string     `json:"responder_id"`
	TargetItemID         string     `json:"target_item_id"`
	Message              *string    `json:"message"`
	Status               Status     `json:"status"`
	ExpiresAt            time.Time  `json:"expires_at"`
	AcceptedAt           *time.Time `json:"accepted_at"`
	InitiatorConfirmedAt *time.Time `json:"initiator_confirmed_at"`
	ResponderConfirmedAt *time.Time `json:"responder_confirmed_at"`
	CreatedAt            time.Time  `json:"created_at"`
	OfferedItemIDs       []string   `json:"-"`
	RequestedItemIDs     []string   `json:"-"`

	// Filled for API responses by enrich.
	InitiatorName string      `json:"initiator_name"`
	ResponderName string      `json:"responder_name"`
	Items         []OfferItem `json:"items"`
}

type OfferItem struct {
	ID        string  `json:"id"`
	Side      string  `json:"side"`
	Title     string  `json:"title"`
	Status    string  `json:"status"`
	CoverPath *string `json:"cover_path"`
}

type CreateOfferInput struct {
	TargetItemID string
	// Initiator's own items, at least one.
	OfferedItemIDs []string
	// Further items of the responder besides the target item. Optional.
	AdditionalRequestedItemIDs []string
	Message                    *string
}

type Service struct {
	pool *pgxpool.Pool
}

func NewService(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool}
}

// CreateOffer enforces invariant 4: the initiator must own at least one active
// item, checked here on the server and never in the app.
func (s *Service) CreateOffer(ctx context.Context, initiatorID string, in CreateOfferInput) (*Offer, error) {
	if !ids.AllUUIDs(append(append([]string{in.TargetItemID}, in.OfferedItemIDs...), in.AdditionalRequestedItemIDs...)) {
		return nil, ErrInvalidInput
	}
	offered := dedupe(in.OfferedItemIDs)
	requested := dedupe(append([]string{in.TargetItemID}, in.AdditionalRequestedItemIDs...))
	if in.TargetItemID == "" || len(offered) == 0 || len(offered) > MaxItemsPerSide || len(requested) > MaxItemsPerSide {
		return nil, ErrInvalidInput
	}
	for _, id := range offered {
		if slices.Contains(requested, id) {
			return nil, ErrInvalidInput
		}
	}

	var offerID string
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		var hasActive bool
		if err := tx.QueryRow(ctx, `
			select exists(select 1 from items where owner_id = $1 and status = 'active')`,
			initiatorID).Scan(&hasActive); err != nil {
			return err
		}
		if !hasActive {
			return ErrNoActiveItem
		}

		// FOR SHARE conflicts with accept's FOR UPDATE, so an offer can never
		// be created on an item that a concurrent acceptance is reserving.
		owners, err := lockItems(ctx, tx, append(slices.Clone(offered), requested...), "for share")
		if err != nil {
			return err
		}
		responderID := owners[in.TargetItemID].ownerID
		for _, id := range offered {
			if it, ok := owners[id]; !ok || it.ownerID != initiatorID || it.status != "active" {
				return ErrItemUnavailable
			}
		}
		for _, id := range requested {
			if it, ok := owners[id]; !ok || it.ownerID != responderID || it.status != "active" {
				return ErrItemUnavailable
			}
		}
		if responderID == initiatorID {
			return ErrInvalidInput
		}
		var responderDeleted bool
		if err := tx.QueryRow(ctx, `select deleted_at is not null from profiles where id = $1`, responderID).Scan(&responderDeleted); err != nil {
			return err
		}
		if responderDeleted {
			return ErrItemUnavailable
		}
		if err := ensureNotBlocked(ctx, tx, initiatorID, responderID); err != nil {
			return err
		}

		if err := tx.QueryRow(ctx, `
			insert into trade_offers (initiator_id, responder_id, target_item_id, message)
			values ($1, $2, $3, $4) returning id`,
			initiatorID, responderID, in.TargetItemID, in.Message).Scan(&offerID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			insert into trade_offer_items (offer_id, item_id, side)
			select $1::uuid, unnest($2::uuid[]), 'offered'::offer_side
			union all
			select $1::uuid, unnest($3::uuid[]), 'requested'::offer_side`,
			offerID, offered, requested); err != nil {
			return err
		}
		return notify(ctx, tx, offerID, NotifyOfferReceived, responderID)
	})
	if err != nil {
		return nil, err
	}
	return s.GetOffer(ctx, initiatorID, offerID)
}

// AcceptOffer implements invariants 1–3 in one transaction: all involved items
// are locked FOR UPDATE, checked, reserved, and every competing pending offer
// on any of them is cancelled with its parties notified.
func (s *Service) AcceptOffer(ctx context.Context, responderID, offerID string) (*Offer, error) {
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		offer, items, err := lockOffer(ctx, tx, offerID)
		if err != nil {
			return err
		}
		if offer.ResponderID != responderID {
			return ErrNotAllowed
		}
		if offer.Status != StatusPending {
			return ErrOfferNotPending
		}
		if !offer.ExpiresAt.After(time.Now()) {
			return ErrOfferExpired
		}
		if err := ensureNotBlocked(ctx, tx, offer.InitiatorID, offer.ResponderID); err != nil {
			return err
		}
		for _, id := range offer.OfferedItemIDs {
			if it := items[id]; it.ownerID != offer.InitiatorID || it.status != "active" {
				return ErrItemUnavailable
			}
		}
		for _, id := range offer.RequestedItemIDs {
			if it := items[id]; it.ownerID != offer.ResponderID || it.status != "active" {
				return ErrItemUnavailable
			}
		}

		allItems := append(slices.Clone(offer.OfferedItemIDs), offer.RequestedItemIDs...)
		if _, err := tx.Exec(ctx, `
			update items set status = 'reserved', reserved_by_offer_id = $1
			where id = any($2::uuid[])`, offerID, allItems); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			update trade_offers set status = 'accepted', accepted_at = now() where id = $1`,
			offerID); err != nil {
			return err
		}
		if err := notify(ctx, tx, offerID, NotifyOfferAccepted, offer.InitiatorID); err != nil {
			return err
		}

		// Competing offers hold only pending state (no reservations), and we
		// already hold every shared item lock, so locking them here cannot
		// deadlock with another acceptance.
		rows, err := tx.Query(ctx, `
			select o.id, o.initiator_id, o.responder_id
			from trade_offers o
			where o.status = 'pending' and o.id <> $1
			  and exists (select 1 from trade_offer_items toi
			              where toi.offer_id = o.id and toi.item_id = any($2::uuid[]))
			order by o.id
			for update`, offerID, allItems)
		if err != nil {
			return err
		}
		type party struct{ id, initiator, responder string }
		competing, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (party, error) {
			var p party
			err := row.Scan(&p.id, &p.initiator, &p.responder)
			return p, err
		})
		if err != nil {
			return err
		}
		for _, c := range competing {
			if _, err := tx.Exec(ctx, `update trade_offers set status = 'cancelled' where id = $1`, c.id); err != nil {
				return err
			}
			if err := notify(ctx, tx, c.id, NotifyOfferCancelled, c.initiator, c.responder); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return s.GetOffer(ctx, responderID, offerID)
}

// DeclineOffer lets the responder turn down a pending offer.
func (s *Service) DeclineOffer(ctx context.Context, responderID, offerID string) (*Offer, error) {
	return s.closeOffer(ctx, responderID, offerID, func(o *Offer) (Status, []string, error) {
		if o.ResponderID != responderID {
			return "", nil, ErrNotAllowed
		}
		if o.Status != StatusPending {
			return "", nil, ErrOfferNotPending
		}
		return StatusDeclined, []string{o.InitiatorID}, nil
	}, NotifyOfferDeclined)
}

// CancelOffer withdraws a pending offer (initiator only) or calls off an
// accepted trade (either party), releasing reserved items.
func (s *Service) CancelOffer(ctx context.Context, userID, offerID string) (*Offer, error) {
	return s.closeOffer(ctx, userID, offerID, func(o *Offer) (Status, []string, error) {
		switch {
		case o.Status == StatusPending && o.InitiatorID == userID:
			return StatusCancelled, []string{o.ResponderID}, nil
		case o.Status == StatusAccepted && (o.InitiatorID == userID || o.ResponderID == userID):
			return StatusCancelled, []string{otherParty(o, userID)}, nil
		case o.InitiatorID != userID && o.ResponderID != userID:
			return "", nil, ErrNotAllowed
		case o.Status == StatusPending:
			return "", nil, ErrNotAllowed // responder declines instead
		default:
			return "", nil, ErrOfferNotPending
		}
	}, NotifyOfferCancelled)
}

// ConfirmTrade records that one party has handed over their items. When both
// have confirmed, the offer completes and all items become traded.
func (s *Service) ConfirmTrade(ctx context.Context, userID, offerID string) (*Offer, error) {
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		offer, _, err := lockOffer(ctx, tx, offerID)
		if err != nil {
			return err
		}
		if offer.InitiatorID != userID && offer.ResponderID != userID {
			return ErrNotAllowed
		}
		if offer.Status != StatusAccepted {
			return ErrOfferNotAccepted
		}
		column, confirmedAt := "initiator_confirmed_at", offer.InitiatorConfirmedAt
		if userID == offer.ResponderID {
			column, confirmedAt = "responder_confirmed_at", offer.ResponderConfirmedAt
		}
		if confirmedAt != nil {
			return ErrAlreadyConfirmed
		}

		var completed bool
		if err := tx.QueryRow(ctx, fmt.Sprintf(`
			update trade_offers set %[1]s = now()
			where id = $1
			returning initiator_confirmed_at is not null and responder_confirmed_at is not null`, column),
			offerID).Scan(&completed); err != nil {
			return err
		}
		if !completed {
			return notify(ctx, tx, offerID, NotifyTradeConfirmed, otherParty(offer, userID))
		}
		if _, err := tx.Exec(ctx, `
			update items set status = 'traded', reserved_by_offer_id = null
			where reserved_by_offer_id = $1`, offerID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `update trade_offers set status = 'completed' where id = $1`, offerID); err != nil {
			return err
		}
		return notify(ctx, tx, offerID, NotifyTradeCompleted, offer.InitiatorID, offer.ResponderID)
	})
	if err != nil {
		return nil, err
	}
	return s.GetOffer(ctx, userID, offerID)
}

// ExpireOffers moves pending offers past their expiry to expired. Each offer
// is handled in its own transaction so one failure does not block the rest.
func (s *Service) ExpireOffers(ctx context.Context) (int, error) {
	rows, err := s.pool.Query(ctx, `
		select id from trade_offers where status = 'pending' and expires_at <= now() order by expires_at limit 500`)
	if err != nil {
		return 0, err
	}
	offerIDs, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return 0, err
	}
	expired := 0
	for _, id := range offerIDs {
		err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
			offer, _, err := lockOffer(ctx, tx, id)
			if err != nil {
				return err
			}
			if offer.Status != StatusPending || offer.ExpiresAt.After(time.Now()) {
				return nil // raced with accept/decline/cancel
			}
			if _, err := tx.Exec(ctx, `update trade_offers set status = 'expired' where id = $1`, id); err != nil {
				return err
			}
			expired++
			return notify(ctx, tx, id, NotifyOfferExpired, offer.InitiatorID, offer.ResponderID)
		})
		if err != nil {
			return expired, fmt.Errorf("expire offer %s: %w", id, err)
		}
	}
	return expired, nil
}

// GetOffer returns an offer visible to userID (one of the two parties).
func (s *Service) GetOffer(ctx context.Context, userID, offerID string) (*Offer, error) {
	if !ids.IsUUID(offerID) {
		return nil, ErrOfferNotFound
	}
	offer, err := loadOffer(ctx, s.pool, offerID, "")
	if err != nil {
		return nil, err
	}
	if offer.InitiatorID != userID && offer.ResponderID != userID {
		return nil, ErrOfferNotFound
	}
	offers := []*Offer{offer}
	if err := s.enrich(ctx, offers); err != nil {
		return nil, err
	}
	return offer, nil
}

// ListOffers returns every offer the user is a party of, newest first.
func (s *Service) ListOffers(ctx context.Context, userID string) ([]*Offer, error) {
	rows, err := s.pool.Query(ctx, `
		select id from trade_offers
		where initiator_id = $1 or responder_id = $1
		order by created_at desc limit 200`, userID)
	if err != nil {
		return nil, err
	}
	offerIDs, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return nil, err
	}
	offers := make([]*Offer, 0, len(offerIDs))
	for _, id := range offerIDs {
		o, err := loadOffer(ctx, s.pool, id, "")
		if err != nil {
			return nil, err
		}
		offers = append(offers, o)
	}
	if err := s.enrich(ctx, offers); err != nil {
		return nil, err
	}
	return offers, nil
}

func (s *Service) enrich(ctx context.Context, offers []*Offer) error {
	for _, o := range offers {
		if err := s.pool.QueryRow(ctx, `
			select (select display_name from profiles where id = $1),
			       (select display_name from profiles where id = $2)`,
			o.InitiatorID, o.ResponderID).Scan(&o.InitiatorName, &o.ResponderName); err != nil {
			return err
		}
		rows, err := s.pool.Query(ctx, `
			select i.id, toi.side::text, i.title, i.status::text,
			       (select storage_path from item_images im where im.item_id = i.id order by position limit 1)
			from trade_offer_items toi join items i on i.id = toi.item_id
			where toi.offer_id = $1
			order by toi.side, i.id = $2 desc, i.title`, o.ID, o.TargetItemID)
		if err != nil {
			return err
		}
		o.Items, err = pgx.CollectRows(rows, func(row pgx.CollectableRow) (OfferItem, error) {
			var it OfferItem
			err := row.Scan(&it.ID, &it.Side, &it.Title, &it.Status, &it.CoverPath)
			return it, err
		})
		if err != nil {
			return err
		}
	}
	return nil
}

// DeleteItem soft-deletes an own draft or active item and then cancels every
// pending offer that includes it. Reserved items cannot be deleted
// (invariant 5). The deletion commits first so that the follow-up
// cancellations can take their locks in the usual items-then-offer order.
func (s *Service) DeleteItem(ctx context.Context, ownerID, itemID string) error {
	if !ids.IsUUID(itemID) {
		return ErrItemNotFound
	}
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		var owner, status string
		err := tx.QueryRow(ctx, `select owner_id, status from items where id = $1 for update`, itemID).Scan(&owner, &status)
		if errors.Is(err, pgx.ErrNoRows) || (err == nil && (owner != ownerID || status == "deleted")) {
			return ErrItemNotFound
		}
		if err != nil {
			return err
		}
		if status != "draft" && status != "active" {
			return ErrItemLocked
		}
		_, err = tx.Exec(ctx, `update items set status = 'deleted' where id = $1`, itemID)
		return err
	})
	if err != nil {
		return err
	}
	return s.cancelOffers(ctx, ownerID, `
		select o.id from trade_offers o join trade_offer_items toi on toi.offer_id = o.id
		where toi.item_id = $1 and o.status = 'pending'`, itemID)
}

// CancelOffersBetween cancels all open (pending or accepted) offers between
// two users, releasing reserved items. Used when one blocks the other.
func (s *Service) CancelOffersBetween(ctx context.Context, actorID, otherID string) error {
	return s.cancelOffers(ctx, actorID, `
		select id from trade_offers
		where status in ('pending', 'accepted')
		  and ((initiator_id = $1 and responder_id = $2) or (initiator_id = $2 and responder_id = $1))`,
		actorID, otherID)
}

// CancelAllOffersOf cancels every open offer of a user. Used on account deletion.
func (s *Service) CancelAllOffersOf(ctx context.Context, userID string) error {
	return s.cancelOffers(ctx, userID, `
		select id from trade_offers
		where status in ('pending', 'accepted') and (initiator_id = $1 or responder_id = $1)`, userID)
}

// cancelOffers cancels each selected offer in its own transaction and notifies
// the other party. Offers that have moved on in the meantime are skipped.
func (s *Service) cancelOffers(ctx context.Context, actorID, selectIDs string, args ...any) error {
	rows, err := s.pool.Query(ctx, selectIDs, args...)
	if err != nil {
		return err
	}
	offerIDs, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return err
	}
	for _, id := range offerIDs {
		err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
			offer, _, err := lockOffer(ctx, tx, id)
			if err != nil {
				return err
			}
			if offer.Status != StatusPending && offer.Status != StatusAccepted {
				return nil
			}
			if err := releaseAndClose(ctx, tx, id, StatusCancelled); err != nil {
				return err
			}
			var recipients []string
			for _, p := range []string{offer.InitiatorID, offer.ResponderID} {
				if p != actorID {
					recipients = append(recipients, p)
				}
			}
			return notify(ctx, tx, id, NotifyOfferCancelled, recipients...)
		})
		if err != nil {
			return fmt.Errorf("cancel offer %s: %w", id, err)
		}
	}
	return nil
}

// closeOffer is the shared path for transitions that end an offer without
// completing it. Leaving pending or accepted without completion always
// releases the reserved items back to active.
func (s *Service) closeOffer(ctx context.Context, userID, offerID string,
	decide func(*Offer) (Status, []string, error), kind string) (*Offer, error) {
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		offer, _, err := lockOffer(ctx, tx, offerID)
		if err != nil {
			return err
		}
		next, recipients, err := decide(offer)
		if err != nil {
			return err
		}
		if err := releaseAndClose(ctx, tx, offerID, next); err != nil {
			return err
		}
		return notify(ctx, tx, offerID, kind, recipients...)
	})
	if err != nil {
		return nil, err
	}
	return s.GetOffer(ctx, userID, offerID)
}

func releaseAndClose(ctx context.Context, tx pgx.Tx, offerID string, next Status) error {
	if _, err := tx.Exec(ctx, `
		update items set status = 'active', reserved_by_offer_id = null
		where reserved_by_offer_id = $1`, offerID); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `update trade_offers set status = $2 where id = $1`, offerID, next)
	return err
}

type lockedItem struct {
	ownerID string
	status  string
}

// lockItems locks the given items in id order and returns them by id. Missing
// ids are simply absent from the map.
func lockItems(ctx context.Context, tx pgx.Tx, itemIDs []string, mode string) (map[string]lockedItem, error) {
	rows, err := tx.Query(ctx, `
		select id, owner_id, status from items
		where id = any($1::uuid[])
		order by id `+mode, itemIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string]lockedItem, len(itemIDs))
	for rows.Next() {
		var id string
		var it lockedItem
		if err := rows.Scan(&id, &it.ownerID, &it.status); err != nil {
			return nil, err
		}
		out[id] = it
	}
	return out, rows.Err()
}

// lockOffer locks the offer's items (FOR UPDATE, by id) and then the offer
// itself, following the package-wide lock order.
func lockOffer(ctx context.Context, tx pgx.Tx, offerID string) (*Offer, map[string]lockedItem, error) {
	if !ids.IsUUID(offerID) {
		return nil, nil, ErrOfferNotFound
	}
	// trade_offer_items never change after creation, so reading them before
	// taking any lock is safe.
	rows, err := tx.Query(ctx, `select item_id from trade_offer_items where offer_id = $1`, offerID)
	if err != nil {
		return nil, nil, err
	}
	itemIDs, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return nil, nil, err
	}
	if len(itemIDs) == 0 {
		return nil, nil, ErrOfferNotFound
	}
	items, err := lockItems(ctx, tx, itemIDs, "for update")
	if err != nil {
		return nil, nil, err
	}
	offer, err := loadOffer(ctx, tx, offerID, "for update")
	if err != nil {
		return nil, nil, err
	}
	return offer, items, nil
}

type querier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func loadOffer(ctx context.Context, q querier, offerID, lock string) (*Offer, error) {
	var o Offer
	err := q.QueryRow(ctx, `
		select id, initiator_id, responder_id, target_item_id, message, status, expires_at,
		       accepted_at, initiator_confirmed_at, responder_confirmed_at, created_at
		from trade_offers where id = $1 `+lock, offerID).Scan(
		&o.ID, &o.InitiatorID, &o.ResponderID, &o.TargetItemID, &o.Message, &o.Status, &o.ExpiresAt,
		&o.AcceptedAt, &o.InitiatorConfirmedAt, &o.ResponderConfirmedAt, &o.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrOfferNotFound
	}
	if err != nil {
		return nil, err
	}
	rows, err := q.Query(ctx, `select item_id, side from trade_offer_items where offer_id = $1 order by item_id`, offerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id, side string
		if err := rows.Scan(&id, &side); err != nil {
			return nil, err
		}
		if side == "offered" {
			o.OfferedItemIDs = append(o.OfferedItemIDs, id)
		} else {
			o.RequestedItemIDs = append(o.RequestedItemIDs, id)
		}
	}
	return &o, rows.Err()
}

func ensureNotBlocked(ctx context.Context, tx pgx.Tx, a, b string) error {
	var blocked bool
	if err := tx.QueryRow(ctx, `
		select exists(select 1 from blocks
		              where (blocker_id = $1 and blocked_id = $2) or (blocker_id = $2 and blocked_id = $1))`,
		a, b).Scan(&blocked); err != nil {
		return err
	}
	if blocked {
		return ErrBlocked
	}
	return nil
}

func notify(ctx context.Context, tx pgx.Tx, offerID, kind string, userIDs ...string) error {
	_, err := tx.Exec(ctx, `
		insert into notifications (user_id, kind, offer_id)
		select unnest($1::uuid[]), $2, $3::uuid`, userIDs, kind, offerID)
	return err
}

func otherParty(o *Offer, userID string) string {
	if o.InitiatorID == userID {
		return o.ResponderID
	}
	return o.InitiatorID
}

func dedupe(values []string) []string {
	out := make([]string, 0, len(values))
	for _, id := range values {
		if id != "" && !slices.Contains(out, id) {
			out = append(out, id)
		}
	}
	return out
}
