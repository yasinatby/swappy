package swaps

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/yasinatby/swappy/backend/internal/items"
	"github.com/yasinatby/swappy/backend/internal/models"
	"github.com/yasinatby/swappy/backend/internal/notifications"
	"github.com/yasinatby/swappy/backend/internal/users"
)

type Service struct {
	db          *sql.DB
	itemService *items.Service
	userService *users.Service
	notifService *notifications.Service
}

func NewService(db *sql.DB, itemService *items.Service, userService *users.Service, notifService *notifications.Service) *Service {
	return &Service{
		db:           db,
		itemService:  itemService,
		userService:  userService,
		notifService: notifService,
	}
}

// Propose initiates a swap proposal.
// Rule 4: A user can only propose a swap if they have at least one active item published.
func (s *Service) Propose(initiatorID string, req models.SwapProposalRequest) (*models.Swap, error) {
	if len(req.OfferedItemIDs) == 0 {
		return nil, errors.New("at least one offered item must be selected")
	}

	// Rule 4 Enforcement:
	activeCount, err := s.itemService.CountActiveByUser(initiatorID)
	if err != nil {
		return nil, fmt.Errorf("failed to check active items: %w", err)
	}
	if activeCount == 0 {
		return nil, errors.New("you must have at least one active item published before you can propose a swap")
	}

	// Fetch target item
	targetItem, err := s.itemService.GetByID(req.TargetItemID, initiatorID)
	if err != nil {
		return nil, errors.New("target item not found")
	}
	if targetItem.UserID == initiatorID {
		return nil, errors.New("cannot propose a swap on your own item")
	}
	if targetItem.Status != models.ItemStatusAvailable {
		return nil, errors.New("target item is not available for swapping")
	}

	recipientID := targetItem.UserID

	// Check if either user has blocked the other
	isBlocked, err := s.userService.IsBlocked(initiatorID, recipientID)
	if err != nil {
		return nil, err
	}
	if isBlocked {
		return nil, errors.New("cannot propose swap with this user")
	}

	// Verify all offered items belong to initiator and are available
	for _, offeredID := range req.OfferedItemIDs {
		offeredItem, err := s.itemService.GetByID(offeredID, initiatorID)
		if err != nil {
			return nil, fmt.Errorf("offered item %s not found", offeredID)
		}
		if offeredItem.UserID != initiatorID {
			return nil, fmt.Errorf("you do not own offered item %s", offeredID)
		}
		if offeredItem.Status != models.ItemStatusAvailable {
			return nil, fmt.Errorf("offered item %s is not available", offeredItem.Title)
		}
	}

	handover := req.HandoverMethod
	if handover != models.HandoverShipping {
		handover = models.HandoverPersonal
	}

	swapID := uuid.New().String()
	now := time.Now()

	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// 1. Insert swap record
	insertSwapQuery := `
	INSERT INTO swaps (
		id, initiator_id, recipient_id, target_item_id, status,
		cash_adjustment, message, handover_method, initiator_confirmed,
		recipient_confirmed, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, 0, 0, ?, ?)
	`
	_, err = tx.Exec(insertSwapQuery,
		swapID, initiatorID, recipientID, req.TargetItemID, models.SwapStatusPending,
		req.CashAdjustment, req.Message, handover, now, now,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to insert swap: %w", err)
	}

	// 2. Insert target item into swap_items
	_, err = tx.Exec("INSERT INTO swap_items (swap_id, item_id, role) VALUES (?, ?, 'target')", swapID, req.TargetItemID)
	if err != nil {
		return nil, fmt.Errorf("failed to associate target item: %w", err)
	}

	// 3. Insert offered items into swap_items
	for _, offeredID := range req.OfferedItemIDs {
		_, err = tx.Exec("INSERT INTO swap_items (swap_id, item_id, role) VALUES (?, ?, 'offered')", swapID, offeredID)
		if err != nil {
			return nil, fmt.Errorf("failed to associate offered item: %w", err)
		}
	}

	// 4. Insert swap history
	offeredJSON, _ := json.Marshal(req.OfferedItemIDs)
	historyID := uuid.New().String()
	historyQuery := `
	INSERT INTO swap_history (id, swap_id, actor_id, action, message, cash_adjustment, offered_item_ids, created_at)
	VALUES (?, ?, ?, 'proposed', ?, ?, ?, ?)
	`
	_, err = tx.Exec(historyQuery, historyID, swapID, initiatorID, req.Message, req.CashAdjustment, string(offeredJSON), now)
	if err != nil {
		return nil, fmt.Errorf("failed to record history: %w", err)
	}

	// 5. Initial chat message if message provided
	if req.Message != "" {
		msgID := uuid.New().String()
		_, err = tx.Exec(`
		INSERT INTO messages (id, swap_id, sender_id, text, is_system, created_at)
		VALUES (?, ?, ?, ?, 0, ?)
		`, msgID, swapID, initiatorID, req.Message, now)
		if err != nil {
			return nil, fmt.Errorf("failed to insert initial message: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	// Send notification to recipient
	initiatorUser, _ := s.userService.GetPublicProfile(initiatorID)
	initName := initiatorID
	if initiatorUser != nil {
		initName = initiatorUser.Username
	}
	_, _ = s.notifService.Create(
		recipientID,
		models.NotifSwapProposal,
		"Neues Tauschangebot! 🔄",
		fmt.Sprintf("%s hat dir ein Tauschangebot für '%s' geschickt.", initName, targetItem.Title),
		"swap",
		swapID,
	)

	return s.GetByID(swapID, initiatorID)
}

func (s *Service) Counter(swapID, actorID string, req models.SwapCounterRequest) (*models.Swap, error) {
	swap, err := s.GetByID(swapID, actorID)
	if err != nil {
		return nil, err
	}

	if swap.Status != models.SwapStatusPending && swap.Status != models.SwapStatusCountered {
		return nil, fmt.Errorf("cannot counter a swap in %s status", swap.Status)
	}

	if len(req.OfferedItemIDs) == 0 {
		return nil, errors.New("at least one offered item must be selected for counter-offer")
	}

	// Determine recipient of the counter offer (the other party)
	var otherPartyID string
	if actorID == swap.InitiatorID {
		otherPartyID = swap.RecipientID
	} else if actorID == swap.RecipientID {
		otherPartyID = swap.InitiatorID
	} else {
		return nil, errors.New("unauthorized: not a participant of this swap")
	}

	// Validate offered items
	for _, itemID := range req.OfferedItemIDs {
		item, err := s.itemService.GetByID(itemID, actorID)
		if err != nil {
			return nil, fmt.Errorf("item %s not found", itemID)
		}
		if item.UserID != actorID && item.UserID != swap.InitiatorID && item.UserID != swap.RecipientID {
			return nil, fmt.Errorf("invalid item ownership for %s", itemID)
		}
	}

	now := time.Now()
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// Update swap record
	_, err = tx.Exec(`
		UPDATE swaps
		SET status = ?, cash_adjustment = ?, message = ?, updated_at = ?
		WHERE id = ?
	`, models.SwapStatusCountered, req.CashAdjustment, req.Message, now, swapID)
	if err != nil {
		return nil, err
	}

	// Replace offered items
	_, _ = tx.Exec("DELETE FROM swap_items WHERE swap_id = ? AND role = 'offered'", swapID)
	for _, offeredID := range req.OfferedItemIDs {
		_, err = tx.Exec("INSERT INTO swap_items (swap_id, item_id, role) VALUES (?, ?, 'offered')", swapID, offeredID)
		if err != nil {
			return nil, err
		}
	}

	// Record history
	offeredJSON, _ := json.Marshal(req.OfferedItemIDs)
	historyID := uuid.New().String()
	_, err = tx.Exec(`
		INSERT INTO swap_history (id, swap_id, actor_id, action, message, cash_adjustment, offered_item_ids, created_at)
		VALUES (?, ?, ?, 'countered', ?, ?, ?, ?)
	`, historyID, swapID, actorID, req.Message, req.CashAdjustment, string(offeredJSON), now)
	if err != nil {
		return nil, err
	}

	// System message in chat
	msgText := fmt.Sprintf("Gegenangebot erstellt: %s", req.Message)
	if req.CashAdjustment != 0 {
		msgText += fmt.Sprintf(" (Wertausgleich: %.2f €)", req.CashAdjustment)
	}
	_, _ = tx.Exec(`
		INSERT INTO messages (id, swap_id, sender_id, text, is_system, created_at)
		VALUES (?, ?, ?, ?, 1, ?)
	`, uuid.New().String(), swapID, actorID, msgText, now)

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	actorProfile, _ := s.userService.GetPublicProfile(actorID)
	actorName := actorID
	if actorProfile != nil {
		actorName = actorProfile.Username
	}
	_, _ = s.notifService.Create(
		otherPartyID,
		models.NotifSwapCounter,
		"Gegenangebot erhalten! ⚖️",
		fmt.Sprintf("%s hat ein Gegenangebot für das Tauschgeschäft gemacht.", actorName),
		"swap",
		swapID,
	)

	return s.GetByID(swapID, actorID)
}

func (s *Service) Accept(swapID, actorID string) (*models.Swap, error) {
	swap, err := s.GetByID(swapID, actorID)
	if err != nil {
		return nil, err
	}

	if swap.Status != models.SwapStatusPending && swap.Status != models.SwapStatusCountered {
		return nil, fmt.Errorf("cannot accept swap in %s status", swap.Status)
	}

	var otherPartyID string
	if actorID == swap.InitiatorID {
		otherPartyID = swap.RecipientID
	} else if actorID == swap.RecipientID {
		otherPartyID = swap.InitiatorID
	} else {
		return nil, errors.New("unauthorized: not a participant of this swap")
	}

	now := time.Now()
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// Update swap to 'agreed' (🔄 Tausch vereinbart)
	_, err = tx.Exec("UPDATE swaps SET status = ?, updated_at = ? WHERE id = ?", models.SwapStatusAgreed, now, swapID)
	if err != nil {
		return nil, err
	}

	// Mark target item as requested 🟡
	_, err = tx.Exec("UPDATE items SET status = ?, updated_at = ? WHERE id = ?", models.ItemStatusRequested, now, swap.TargetItemID)
	if err != nil {
		return nil, err
	}

	// Mark offered items as requested 🟡
	for _, offered := range swap.OfferedItems {
		_, err = tx.Exec("UPDATE items SET status = ?, updated_at = ? WHERE id = ?", models.ItemStatusRequested, now, offered.ID)
		if err != nil {
			return nil, err
		}
	}

	// Record history
	historyID := uuid.New().String()
	_, err = tx.Exec(`
		INSERT INTO swap_history (id, swap_id, actor_id, action, message, cash_adjustment, offered_item_ids, created_at)
		VALUES (?, ?, ?, 'accepted', 'Tauschvorschlag angenommen', ?, '[]', ?)
	`, historyID, swapID, actorID, swap.CashAdjustment, now)
	if err != nil {
		return nil, err
	}

	// System message in chat
	_, _ = tx.Exec(`
		INSERT INTO messages (id, swap_id, sender_id, text, is_system, created_at)
		VALUES (?, ?, ?, 'Tauschangebot wurde angenommen! Status: 🔄 Tausch vereinbart.', 1, ?)
	`, uuid.New().String(), swapID, actorID, now)

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	_, _ = s.notifService.Create(
		otherPartyID,
		models.NotifSwapAccepted,
		"Tausch angenommen! 🎉",
		"Dein Tauschvorschlag wurde angenommen! Bitte stimme die Übergabe oder den Versand im Chat ab.",
		"swap",
		swapID,
	)

	return s.GetByID(swapID, actorID)
}

func (s *Service) Reject(swapID, actorID string) (*models.Swap, error) {
	swap, err := s.GetByID(swapID, actorID)
	if err != nil {
		return nil, err
	}

	var otherPartyID string
	if actorID == swap.InitiatorID {
		otherPartyID = swap.RecipientID
	} else if actorID == swap.RecipientID {
		otherPartyID = swap.InitiatorID
	} else {
		return nil, errors.New("unauthorized: not a participant of this swap")
	}

	now := time.Now()
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// Update swap to rejected
	_, err = tx.Exec("UPDATE swaps SET status = ?, updated_at = ? WHERE id = ?", models.SwapStatusRejected, now, swapID)
	if err != nil {
		return nil, err
	}

	// Restore items to available if they were requested
	if swap.Status == models.SwapStatusAgreed {
		_, _ = tx.Exec("UPDATE items SET status = 'available' WHERE id = ?", swap.TargetItemID)
		for _, offered := range swap.OfferedItems {
			_, _ = tx.Exec("UPDATE items SET status = 'available' WHERE id = ?", offered.ID)
		}
	}

	// Record history
	historyID := uuid.New().String()
	_, _ = tx.Exec(`
		INSERT INTO swap_history (id, swap_id, actor_id, action, message, cash_adjustment, offered_item_ids, created_at)
		VALUES (?, ?, ?, 'rejected', 'Tausch abgelehnt', 0, '[]', ?)
	`, historyID, swapID, actorID, now)

	// System message
	_, _ = tx.Exec(`
		INSERT INTO messages (id, swap_id, sender_id, text, is_system, created_at)
		VALUES (?, ?, ?, 'Das Tauschangebot wurde abgelehnt.', 1, ?)
	`, uuid.New().String(), swapID, actorID, now)

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	_, _ = s.notifService.Create(
		otherPartyID,
		models.NotifSwapRejected,
		"Tausch abgelehnt",
		"Das Tauschangebot wurde abgelehnt.",
		"swap",
		swapID,
	)

	return s.GetByID(swapID, actorID)
}

// Complete handles confirmation and completion of a swap.
// When both parties confirm (or directly completed), items become 'traded' 🔴 and user trade counts increment.
func (s *Service) Complete(swapID, actorID string) (*models.Swap, error) {
	swap, err := s.GetByID(swapID, actorID)
	if err != nil {
		return nil, err
	}

	if swap.Status != models.SwapStatusAgreed && swap.Status != models.SwapStatusCompleted {
		return nil, errors.New("swap must be in agreed status before completing")
	}

	var initConfirmed = swap.InitiatorConfirmed
	var recConfirmed = swap.RecipientConfirmed

	if actorID == swap.InitiatorID {
		initConfirmed = true
	} else if actorID == swap.RecipientID {
		recConfirmed = true
	} else {
		return nil, errors.New("unauthorized: not a participant of this swap")
	}

	now := time.Now()
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// Update confirmation flags
	_, err = tx.Exec(`
		UPDATE swaps
		SET initiator_confirmed = ?, recipient_confirmed = ?, updated_at = ?
		WHERE id = ?
	`, initConfirmed, recConfirmed, now, swapID)
	if err != nil {
		return nil, err
	}

	// If both confirmed (or if completing):
	if initConfirmed && recConfirmed {
		// Mark swap as completed
		_, err = tx.Exec("UPDATE swaps SET status = ?, updated_at = ? WHERE id = ?", models.SwapStatusCompleted, now, swapID)
		if err != nil {
			return nil, err
		}

		// Mark target item as traded 🔴
		_, err = tx.Exec("UPDATE items SET status = ?, updated_at = ? WHERE id = ?", models.ItemStatusTraded, now, swap.TargetItemID)
		if err != nil {
			return nil, err
		}

		// Mark all offered items as traded 🔴
		for _, offered := range swap.OfferedItems {
			_, err = tx.Exec("UPDATE items SET status = ?, updated_at = ? WHERE id = ?", models.ItemStatusTraded, now, offered.ID)
			if err != nil {
				return nil, err
			}
		}

		// Increment completed_trades_count for both users
		_, err = tx.Exec("UPDATE users SET completed_trades_count = completed_trades_count + 1 WHERE id IN (?, ?)", swap.InitiatorID, swap.RecipientID)
		if err != nil {
			return nil, err
		}

		// Record history
		historyID := uuid.New().String()
		_, _ = tx.Exec(`
			INSERT INTO swap_history (id, swap_id, actor_id, action, message, cash_adjustment, offered_item_ids, created_at)
			VALUES (?, ?, ?, 'completed', 'Tausch erfolgreich abgeschlossen', ?, '[]', ?)
		`, historyID, swapID, actorID, swap.CashAdjustment, now)

		// System message
		_, _ = tx.Exec(`
			INSERT INTO messages (id, swap_id, sender_id, text, is_system, created_at)
			VALUES (?, ?, ?, '🎉 Tausch erfolgreich abgeschlossen! Bitte bewerten Sie Ihren Tauschpartner ⭐', 1, ?)
		`, uuid.New().String(), swapID, actorID, now)
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	if initConfirmed && recConfirmed {
		_, _ = s.notifService.Create(
			swap.InitiatorID,
			models.NotifSwapCompleted,
			"Tausch abgeschlossen! 🤝",
			"Der Tausch wurde erfolgreich abgeschlossen. Du kannst nun deinen Partner bewerten.",
			"swap",
			swapID,
		)
		_, _ = s.notifService.Create(
			swap.RecipientID,
			models.NotifSwapCompleted,
			"Tausch abgeschlossen! 🤝",
			"Der Tausch wurde erfolgreich abgeschlossen. Du kannst nun deinen Partner bewerten.",
			"swap",
			swapID,
		)
	}

	return s.GetByID(swapID, actorID)
}

func (s *Service) GetByID(id string, currentUserID string) (*models.Swap, error) {
	query := `
	SELECT s.id, s.initiator_id, s.recipient_id, s.target_item_id, s.status,
	       s.cash_adjustment, s.message, s.handover_method, s.initiator_confirmed,
	       s.recipient_confirmed, s.created_at, s.updated_at
	FROM swaps s
	WHERE s.id = ?
	`
	row := s.db.QueryRow(query, id)

	var swap models.Swap
	var message sql.NullString
	var initConf, recConf int

	err := row.Scan(
		&swap.ID, &swap.InitiatorID, &swap.RecipientID, &swap.TargetItemID, &swap.Status,
		&swap.CashAdjustment, &message, &swap.HandoverMethod, &initConf, &recConf,
		&swap.CreatedAt, &swap.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("swap not found")
		}
		return nil, err
	}

	swap.Message = message.String
	swap.InitiatorConfirmed = initConf > 0
	swap.RecipientConfirmed = recConf > 0

	// Fetch Target Item
	if target, err := s.itemService.GetByID(swap.TargetItemID, currentUserID); err == nil {
		swap.TargetItem = target
	}

	// Fetch Offered Items
	offeredRows, err := s.db.Query(`
		SELECT item_id FROM swap_items WHERE swap_id = ? AND role = 'offered'
	`, swap.ID)
	if err == nil {
		var offeredIDs []string
		for offeredRows.Next() {
			var oID string
			if err := offeredRows.Scan(&oID); err == nil {
				offeredIDs = append(offeredIDs, oID)
			}
		}
		offeredRows.Close()

		var offeredItems []models.Item
		for _, oID := range offeredIDs {
			if item, err := s.itemService.GetByID(oID, currentUserID); err == nil {
				offeredItems = append(offeredItems, *item)
			}
		}
		swap.OfferedItems = offeredItems
	}

	// Fetch Profiles
	swap.Initiator, _ = s.userService.GetPublicProfile(swap.InitiatorID)
	swap.Recipient, _ = s.userService.GetPublicProfile(swap.RecipientID)

	// Fetch History
	historyRows, err := s.db.Query(`
		SELECT sh.id, sh.swap_id, sh.actor_id, u.username, sh.action, sh.message,
		       sh.cash_adjustment, sh.offered_item_ids, sh.created_at
		FROM swap_history sh
		JOIN users u ON u.id = sh.actor_id
		WHERE sh.swap_id = ?
		ORDER BY sh.created_at ASC
	`, swap.ID)
	if err == nil {
		var histories []models.SwapHistory
		for historyRows.Next() {
			var h models.SwapHistory
			var hMsg sql.NullString
			var offeredRaw string
			if err := historyRows.Scan(&h.ID, &h.SwapID, &h.ActorID, &h.ActorUsername, &h.Action, &hMsg, &h.CashAdjustment, &offeredRaw, &h.CreatedAt); err == nil {
				h.Message = hMsg.String
				_ = json.Unmarshal([]byte(offeredRaw), &h.OfferedItemIDs)
				histories = append(histories, h)
			}
		}
		historyRows.Close()
		swap.History = histories
	}

	return &swap, nil
}

func (s *Service) ListUserSwaps(userID string, statusFilter string) ([]models.Swap, error) {
	query := `
	SELECT s.id, s.initiator_id, s.recipient_id, s.target_item_id, s.status,
	       s.cash_adjustment, s.message, s.handover_method, s.initiator_confirmed,
	       s.recipient_confirmed, s.created_at, s.updated_at
	FROM swaps s
	WHERE (s.initiator_id = ? OR s.recipient_id = ?)
	`
	var args []any
	args = append(args, userID, userID)

	if statusFilter != "" {
		query += " AND s.status = ?"
		args = append(args, statusFilter)
	}

	query += " ORDER BY s.updated_at DESC"

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}

	var swaps []models.Swap
	for rows.Next() {
		var swap models.Swap
		var message sql.NullString
		var initConf, recConf int

		if err := rows.Scan(
			&swap.ID, &swap.InitiatorID, &swap.RecipientID, &swap.TargetItemID, &swap.Status,
			&swap.CashAdjustment, &message, &swap.HandoverMethod, &initConf, &recConf,
			&swap.CreatedAt, &swap.UpdatedAt,
		); err != nil {
			rows.Close()
			return nil, err
		}

		swap.Message = message.String
		swap.InitiatorConfirmed = initConf > 0
		swap.RecipientConfirmed = recConf > 0
		swaps = append(swaps, swap)
	}
	rows.Close()

	// Fill in target item and user profiles after closing rows
	for i := range swaps {
		if target, err := s.itemService.GetByID(swaps[i].TargetItemID, userID); err == nil {
			swaps[i].TargetItem = target
		}
		swaps[i].Initiator, _ = s.userService.GetPublicProfile(swaps[i].InitiatorID)
		swaps[i].Recipient, _ = s.userService.GetPublicProfile(swaps[i].RecipientID)
	}

	return swaps, nil
}
