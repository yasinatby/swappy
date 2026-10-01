package swaps_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/yasinatby/swappy/backend/internal/swaps"
	"github.com/yasinatby/swappy/backend/internal/testdb"
)

type fixture struct {
	t    *testing.T
	ctx  context.Context
	pool *pgxpool.Pool
	svc  *swaps.Service
	n    int
}

func newFixture(t *testing.T) *fixture {
	pool := testdb.New(t)
	return &fixture{t: t, ctx: context.Background(), pool: pool, svc: swaps.NewService(pool)}
}

func (f *fixture) user() string {
	f.t.Helper()
	f.n++
	var id string
	err := f.pool.QueryRow(f.ctx, `
		with u as (insert into users (email, password_hash) values ($1, 'x') returning id)
		insert into profiles (id, display_name, postal_code, terms_accepted_at)
		select id, $2, '10115', now() from u returning id`,
		fmt.Sprintf("u%d@test", f.n), fmt.Sprintf("user%d", f.n)).Scan(&id)
	if err != nil {
		f.t.Fatal(err)
	}
	return id
}

func (f *fixture) item(ownerID, status string) string {
	f.t.Helper()
	var id string
	err := f.pool.QueryRow(f.ctx, `
		insert into items (owner_id, title, category, condition, status)
		values ($1, 'Ding', 'misc', 'good', $2) returning id`, ownerID, status).Scan(&id)
	if err != nil {
		f.t.Fatal(err)
	}
	return id
}

func (f *fixture) offer(initiator, target string, offered ...string) *swaps.Offer {
	f.t.Helper()
	o, err := f.svc.CreateOffer(f.ctx, initiator, swaps.CreateOfferInput{TargetItemID: target, OfferedItemIDs: offered})
	if err != nil {
		f.t.Fatalf("create offer: %v", err)
	}
	return o
}

func (f *fixture) itemStatus(id string) string {
	f.t.Helper()
	var s string
	if err := f.pool.QueryRow(f.ctx, `select status from items where id = $1`, id).Scan(&s); err != nil {
		f.t.Fatal(err)
	}
	return s
}

func (f *fixture) offerStatus(id string) swaps.Status {
	f.t.Helper()
	var s swaps.Status
	if err := f.pool.QueryRow(f.ctx, `select status from trade_offers where id = $1`, id).Scan(&s); err != nil {
		f.t.Fatal(err)
	}
	return s
}

func (f *fixture) count(sql string, args ...any) int {
	f.t.Helper()
	var n int
	if err := f.pool.QueryRow(f.ctx, sql, args...).Scan(&n); err != nil {
		f.t.Fatal(err)
	}
	return n
}

func wantErr(t *testing.T, got, want error) {
	t.Helper()
	if !errors.Is(got, want) {
		t.Fatalf("got error %v, want %v", got, want)
	}
}

// Invariant 4.
func TestCreateOfferRequiresActiveItem(t *testing.T) {
	f := newFixture(t)
	initiator, responder := f.user(), f.user()
	draft := f.item(initiator, "draft")
	target := f.item(responder, "active")

	_, err := f.svc.CreateOffer(f.ctx, initiator, swaps.CreateOfferInput{TargetItemID: target, OfferedItemIDs: []string{draft}})
	wantErr(t, err, swaps.ErrNoActiveItem)
}

func TestCreateOfferRejectsUnavailableItems(t *testing.T) {
	f := newFixture(t)
	initiator, responder, other := f.user(), f.user(), f.user()
	mine := f.item(initiator, "active")
	target := f.item(responder, "active")
	foreign := f.item(other, "active")
	responderDraft := f.item(responder, "draft")

	cases := map[string]swaps.CreateOfferInput{
		"offers someone else's item":  {TargetItemID: target, OfferedItemIDs: []string{foreign}},
		"requests a draft":            {TargetItemID: target, OfferedItemIDs: []string{mine}, AdditionalRequestedItemIDs: []string{responderDraft}},
		"requests a third party item": {TargetItemID: target, OfferedItemIDs: []string{mine}, AdditionalRequestedItemIDs: []string{foreign}},
		"targets an own item":         {TargetItemID: mine, OfferedItemIDs: []string{mine}},
		"target does not exist":       {TargetItemID: "00000000-0000-4000-8000-000000000000", OfferedItemIDs: []string{mine}},
	}
	for name, in := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := f.svc.CreateOffer(f.ctx, initiator, in)
			if err == nil {
				t.Fatal("expected an error")
			}
		})
	}

	_, err := f.svc.CreateOffer(f.ctx, initiator, swaps.CreateOfferInput{TargetItemID: "not-a-uuid", OfferedItemIDs: []string{mine}})
	wantErr(t, err, swaps.ErrInvalidInput)
	_, err = f.svc.CreateOffer(f.ctx, initiator, swaps.CreateOfferInput{TargetItemID: target})
	wantErr(t, err, swaps.ErrInvalidInput)
}

// Invariant 5: reserved items cannot be used in another offer.
func TestCreateOfferRejectsReservedItem(t *testing.T) {
	f := newFixture(t)
	a, b, c := f.user(), f.user(), f.user()
	aItem, bItem := f.item(a, "active"), f.item(b, "active")
	f.item(c, "active")
	o := f.offer(a, bItem, aItem)
	if _, err := f.svc.AcceptOffer(f.ctx, b, o.ID); err != nil {
		t.Fatal(err)
	}

	cItem := f.item(c, "active")
	_, err := f.svc.CreateOffer(f.ctx, c, swaps.CreateOfferInput{TargetItemID: bItem, OfferedItemIDs: []string{cItem}})
	wantErr(t, err, swaps.ErrItemUnavailable)
}

// Invariant 6.
func TestBlockPreventsOffersBothWays(t *testing.T) {
	f := newFixture(t)
	a, b := f.user(), f.user()
	aItem, bItem := f.item(a, "active"), f.item(b, "active")
	if _, err := f.pool.Exec(f.ctx, `insert into blocks (blocker_id, blocked_id) values ($1, $2)`, b, a); err != nil {
		t.Fatal(err)
	}

	_, err := f.svc.CreateOffer(f.ctx, a, swaps.CreateOfferInput{TargetItemID: bItem, OfferedItemIDs: []string{aItem}})
	wantErr(t, err, swaps.ErrBlocked)
	_, err = f.svc.CreateOffer(f.ctx, b, swaps.CreateOfferInput{TargetItemID: aItem, OfferedItemIDs: []string{bItem}})
	wantErr(t, err, swaps.ErrBlocked)
}

// Invariants 1–3 on the happy path.
func TestAcceptReservesItemsAndCancelsCompetitors(t *testing.T) {
	f := newFixture(t)
	a, b, c := f.user(), f.user(), f.user()
	aItem, bItem, bExtra, cItem := f.item(a, "active"), f.item(b, "active"), f.item(b, "active"), f.item(c, "active")

	winner, err := f.svc.CreateOffer(f.ctx, a, swaps.CreateOfferInput{
		TargetItemID: bItem, OfferedItemIDs: []string{aItem}, AdditionalRequestedItemIDs: []string{bExtra},
	})
	if err != nil {
		t.Fatal(err)
	}
	competitor := f.offer(c, bExtra, cItem) // overlaps on bExtra
	unrelated := f.offer(c, aItem, cItem)   // overlaps on aItem from the other side

	accepted, err := f.svc.AcceptOffer(f.ctx, b, winner.ID)
	if err != nil {
		t.Fatal(err)
	}
	if accepted.Status != swaps.StatusAccepted || accepted.AcceptedAt == nil {
		t.Fatalf("offer not accepted: %+v", accepted)
	}
	for _, id := range []string{aItem, bItem, bExtra} {
		if s := f.itemStatus(id); s != "reserved" {
			t.Errorf("item %s is %s, want reserved", id, s)
		}
	}
	if s := f.itemStatus(cItem); s != "active" {
		t.Errorf("uninvolved item is %s", s)
	}
	if s := f.offerStatus(competitor.ID); s != swaps.StatusCancelled {
		t.Errorf("competitor is %s, want cancelled", s)
	}
	if s := f.offerStatus(unrelated.ID); s != swaps.StatusCancelled {
		t.Errorf("offer on initiator's item is %s, want cancelled", s)
	}
	if n := f.count(`select count(*) from notifications where offer_id = $1 and kind = 'offer_cancelled' and user_id = $2`, competitor.ID, c); n != 1 {
		t.Errorf("competitor initiator got %d cancel notifications", n)
	}
	if n := f.count(`select count(*) from notifications where offer_id = $1 and kind = 'offer_accepted' and user_id = $2`, winner.ID, a); n != 1 {
		t.Errorf("initiator got %d accept notifications", n)
	}
}

func TestAcceptErrors(t *testing.T) {
	f := newFixture(t)
	a, b := f.user(), f.user()
	aItem, bItem := f.item(a, "active"), f.item(b, "active")
	o := f.offer(a, bItem, aItem)

	_, err := f.svc.AcceptOffer(f.ctx, a, o.ID)
	wantErr(t, err, swaps.ErrNotAllowed)
	_, err = f.svc.AcceptOffer(f.ctx, b, "00000000-0000-4000-8000-000000000000")
	wantErr(t, err, swaps.ErrOfferNotFound)
	_, err = f.svc.AcceptOffer(f.ctx, b, "garbage")
	wantErr(t, err, swaps.ErrOfferNotFound)

	if _, err := f.pool.Exec(f.ctx, `update trade_offers set expires_at = now() - interval '1 second' where id = $1`, o.ID); err != nil {
		t.Fatal(err)
	}
	_, err = f.svc.AcceptOffer(f.ctx, b, o.ID)
	wantErr(t, err, swaps.ErrOfferExpired)
}

// Invariant 2: two parallel acceptances of the same item — exactly one wins,
// the others get a clean domain error (no deadlock, no serialization error).
func TestParallelAcceptOfSameItem(t *testing.T) {
	f := newFixture(t)
	responder := f.user()
	target := f.item(responder, "active")

	const n = 12
	offers := make([]string, n)
	for i := range offers {
		initiator := f.user()
		offers[i] = f.offer(initiator, target, f.item(initiator, "active")).ID
	}

	errs := runParallel(n, func(i int) error {
		_, err := f.svc.AcceptOffer(f.ctx, responder, offers[i])
		return err
	})

	wins := 0
	for i, err := range errs {
		switch {
		case err == nil:
			wins++
		case errors.Is(err, swaps.ErrOfferNotPending), errors.Is(err, swaps.ErrItemUnavailable):
		default:
			t.Errorf("offer %d: unexpected error %v", i, err)
		}
	}
	if wins != 1 {
		t.Fatalf("%d acceptances won, want exactly 1", wins)
	}
	assertNoDoubleAcceptance(t, f)
}

// Overlapping offers listing the same items in different order must not
// deadlock; again exactly one wins.
func TestParallelAcceptOfCrossOverlappingOffers(t *testing.T) {
	f := newFixture(t)
	responder := f.user()
	x, y := f.item(responder, "active"), f.item(responder, "active")

	for round := 0; round < 10; round++ {
		a, c := f.user(), f.user()
		o1, err := f.svc.CreateOffer(f.ctx, a, swaps.CreateOfferInput{
			TargetItemID: x, AdditionalRequestedItemIDs: []string{y}, OfferedItemIDs: []string{f.item(a, "active")},
		})
		if err != nil {
			t.Fatal(err)
		}
		o2, err := f.svc.CreateOffer(f.ctx, c, swaps.CreateOfferInput{
			TargetItemID: y, AdditionalRequestedItemIDs: []string{x}, OfferedItemIDs: []string{f.item(c, "active")},
		})
		if err != nil {
			t.Fatal(err)
		}

		ids := []string{o1.ID, o2.ID}
		errs := runParallel(2, func(i int) error {
			_, err := f.svc.AcceptOffer(f.ctx, responder, ids[i])
			return err
		})
		wins := 0
		for _, err := range errs {
			if err == nil {
				wins++
			} else if !errors.Is(err, swaps.ErrOfferNotPending) && !errors.Is(err, swaps.ErrItemUnavailable) {
				t.Fatalf("round %d: unexpected error %v", round, err)
			}
		}
		if wins != 1 {
			t.Fatalf("round %d: %d wins", round, wins)
		}
		assertNoDoubleAcceptance(t, f)

		// Free the items for the next round.
		winner := o1.ID
		if errs[0] != nil {
			winner = o2.ID
		}
		if _, err := f.svc.CancelOffer(f.ctx, responder, winner); err != nil {
			t.Fatal(err)
		}
	}
}

func TestParallelAcceptOfSameOffer(t *testing.T) {
	f := newFixture(t)
	a, b := f.user(), f.user()
	o := f.offer(a, f.item(b, "active"), f.item(a, "active"))

	errs := runParallel(5, func(int) error {
		_, err := f.svc.AcceptOffer(f.ctx, b, o.ID)
		return err
	})
	wins := 0
	for _, err := range errs {
		if err == nil {
			wins++
		} else {
			wantErr(t, err, swaps.ErrOfferNotPending)
		}
	}
	if wins != 1 {
		t.Fatalf("%d wins", wins)
	}
}

func TestCancelAcceptedReleasesItems(t *testing.T) {
	f := newFixture(t)
	a, b := f.user(), f.user()
	aItem, bItem := f.item(a, "active"), f.item(b, "active")
	o := f.offer(a, bItem, aItem)
	if _, err := f.svc.AcceptOffer(f.ctx, b, o.ID); err != nil {
		t.Fatal(err)
	}

	cancelled, err := f.svc.CancelOffer(f.ctx, a, o.ID)
	if err != nil {
		t.Fatal(err)
	}
	if cancelled.Status != swaps.StatusCancelled {
		t.Fatalf("status %s", cancelled.Status)
	}
	for _, id := range []string{aItem, bItem} {
		if s := f.itemStatus(id); s != "active" {
			t.Errorf("item %s is %s after cancel, want active", id, s)
		}
	}
	_, err = f.svc.CancelOffer(f.ctx, a, o.ID)
	wantErr(t, err, swaps.ErrOfferNotPending)
}

func TestDeclineAndCancelPermissions(t *testing.T) {
	f := newFixture(t)
	a, b, stranger := f.user(), f.user(), f.user()
	o := f.offer(a, f.item(b, "active"), f.item(a, "active"))

	_, err := f.svc.DeclineOffer(f.ctx, a, o.ID)
	wantErr(t, err, swaps.ErrNotAllowed)
	_, err = f.svc.CancelOffer(f.ctx, b, o.ID)
	wantErr(t, err, swaps.ErrNotAllowed)
	_, err = f.svc.CancelOffer(f.ctx, stranger, o.ID)
	wantErr(t, err, swaps.ErrNotAllowed)
	_, err = f.svc.GetOffer(f.ctx, stranger, o.ID)
	wantErr(t, err, swaps.ErrOfferNotFound)

	declined, err := f.svc.DeclineOffer(f.ctx, b, o.ID)
	if err != nil {
		t.Fatal(err)
	}
	if declined.Status != swaps.StatusDeclined {
		t.Fatalf("status %s", declined.Status)
	}
	_, err = f.svc.AcceptOffer(f.ctx, b, o.ID)
	wantErr(t, err, swaps.ErrOfferNotPending)
}

func TestConfirmTradeCompletesAfterBothParties(t *testing.T) {
	f := newFixture(t)
	a, b := f.user(), f.user()
	aItem, bItem := f.item(a, "active"), f.item(b, "active")
	o := f.offer(a, bItem, aItem)

	_, err := f.svc.ConfirmTrade(f.ctx, a, o.ID)
	wantErr(t, err, swaps.ErrOfferNotAccepted)

	if _, err := f.svc.AcceptOffer(f.ctx, b, o.ID); err != nil {
		t.Fatal(err)
	}
	half, err := f.svc.ConfirmTrade(f.ctx, a, o.ID)
	if err != nil {
		t.Fatal(err)
	}
	if half.Status != swaps.StatusAccepted || half.InitiatorConfirmedAt == nil {
		t.Fatalf("after first confirm: %+v", half)
	}
	_, err = f.svc.ConfirmTrade(f.ctx, a, o.ID)
	wantErr(t, err, swaps.ErrAlreadyConfirmed)

	done, err := f.svc.ConfirmTrade(f.ctx, b, o.ID)
	if err != nil {
		t.Fatal(err)
	}
	if done.Status != swaps.StatusCompleted {
		t.Fatalf("status %s, want completed", done.Status)
	}
	for _, id := range []string{aItem, bItem} {
		if s := f.itemStatus(id); s != "traded" {
			t.Errorf("item %s is %s, want traded", id, s)
		}
	}
	_, err = f.svc.CancelOffer(f.ctx, a, o.ID)
	wantErr(t, err, swaps.ErrOfferNotPending)
}

func TestExpireOffers(t *testing.T) {
	f := newFixture(t)
	a, b := f.user(), f.user()
	stale := f.offer(a, f.item(b, "active"), f.item(a, "active"))
	fresh := f.offer(a, f.item(b, "active"), f.item(a, "active"))
	if _, err := f.pool.Exec(f.ctx, `update trade_offers set expires_at = now() - interval '1 minute' where id = $1`, stale.ID); err != nil {
		t.Fatal(err)
	}

	n, err := f.svc.ExpireOffers(f.ctx)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("expired %d offers, want 1", n)
	}
	if s := f.offerStatus(stale.ID); s != swaps.StatusExpired {
		t.Errorf("stale offer is %s", s)
	}
	if s := f.offerStatus(fresh.ID); s != swaps.StatusPending {
		t.Errorf("fresh offer is %s", s)
	}
}

func TestDefaultExpiryIsFourteenDays(t *testing.T) {
	f := newFixture(t)
	a, b := f.user(), f.user()
	o := f.offer(a, f.item(b, "active"), f.item(a, "active"))
	days := f.count(`select round(extract(epoch from expires_at - created_at) / 86400)::int from trade_offers where id = $1`, o.ID)
	if days != 14 {
		t.Fatalf("expiry is %d days", days)
	}
}

// The database itself refuses to edit or delete a reserved item (invariant 5).
func TestReservedItemIsFrozenInDatabase(t *testing.T) {
	f := newFixture(t)
	a, b := f.user(), f.user()
	aItem, bItem := f.item(a, "active"), f.item(b, "active")
	o := f.offer(a, bItem, aItem)
	if _, err := f.svc.AcceptOffer(f.ctx, b, o.ID); err != nil {
		t.Fatal(err)
	}

	for name, sql := range map[string]string{
		"edit title":  `update items set title = 'neu' where id = $1`,
		"soft delete": `update items set status = 'deleted', reserved_by_offer_id = null where id = $1`,
	} {
		if _, err := f.pool.Exec(f.ctx, sql, aItem); err == nil {
			t.Errorf("%s on reserved item succeeded", name)
		}
	}
}

func runParallel(n int, fn func(i int) error) []error {
	errs := make([]error, n)
	var start, done sync.WaitGroup
	start.Add(1)
	for i := 0; i < n; i++ {
		done.Add(1)
		go func() {
			defer done.Done()
			start.Wait()
			errs[i] = fn(i)
		}()
	}
	start.Done()
	done.Wait()
	return errs
}

// Invariant 1, checked directly on the data.
func assertNoDoubleAcceptance(t *testing.T, f *fixture) {
	t.Helper()
	n := f.count(`
		select count(*) from (
			select toi.item_id from trade_offer_items toi
			join trade_offers o on o.id = toi.offer_id
			where o.status = 'accepted'
			group by toi.item_id having count(*) > 1
		) dup`)
	if n != 0 {
		t.Fatalf("%d items are in more than one accepted offer", n)
	}
}
