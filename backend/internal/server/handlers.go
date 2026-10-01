package server

import (
	"net/http"
	"strconv"

	"github.com/yasinatby/swappy/backend/internal/apperr"
	"github.com/yasinatby/swappy/backend/internal/items"
	"github.com/yasinatby/swappy/backend/internal/swaps"
	"github.com/yasinatby/swappy/backend/internal/users"
)

// --- dev login -------------------------------------------------------------

func (a *api) devProfiles(w http.ResponseWriter, r *http.Request) {
	profiles, err := a.Users.DevProfiles(r.Context())
	respond(w, http.StatusOK, map[string]any{"profiles": profiles}, err)
}

func (a *api) devLogin(w http.ResponseWriter, r *http.Request) {
	var in struct {
		DisplayName string `json:"display_name"`
	}
	if err := readJSON(r, &in); err != nil {
		writeError(w, err)
		return
	}
	profile, err := a.Users.DevLogin(r.Context(), in.DisplayName)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"token": a.Tokens.Issue(profile.ID), "profile": profile})
}

// --- me --------------------------------------------------------------------

func (a *api) getMe(w http.ResponseWriter, r *http.Request) {
	profile, err := a.Users.Me(r.Context(), me(r))
	respond(w, http.StatusOK, profile, err)
}

func (a *api) updateMe(w http.ResponseWriter, r *http.Request) {
	var in users.ProfileInput
	if err := readJSON(r, &in); err != nil {
		writeError(w, err)
		return
	}
	profile, err := a.Users.UpdateMe(r.Context(), me(r), in)
	respond(w, http.StatusOK, profile, err)
}

func (a *api) deleteMe(w http.ResponseWriter, r *http.Request) {
	respond(w, http.StatusNoContent, nil, a.Users.DeleteAccount(r.Context(), me(r)))
}

func (a *api) myItems(w http.ResponseWriter, r *http.Request) {
	list, err := a.Items.Mine(r.Context(), me(r))
	respond(w, http.StatusOK, map[string]any{"items": list}, err)
}

func (a *api) myBlocks(w http.ResponseWriter, r *http.Request) {
	list, err := a.Users.Blocked(r.Context(), me(r))
	respond(w, http.StatusOK, map[string]any{"blocked": list}, err)
}

// --- items -----------------------------------------------------------------

func (a *api) feed(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	offset, _ := strconv.Atoi(q.Get("offset"))
	list, err := a.Items.Feed(r.Context(), me(r), items.FeedQuery{
		Search: q.Get("q"), Category: q.Get("category"), OwnerID: q.Get("owner"), Offset: offset,
	})
	respond(w, http.StatusOK, map[string]any{"items": list}, err)
}

func (a *api) getItem(w http.ResponseWriter, r *http.Request) {
	item, err := a.Items.Get(r.Context(), me(r), r.PathValue("id"))
	respond(w, http.StatusOK, item, err)
}

func (a *api) createItem(w http.ResponseWriter, r *http.Request) {
	var in struct {
		items.Input
		Publish bool `json:"publish"`
	}
	if err := readJSON(r, &in); err != nil {
		writeError(w, err)
		return
	}
	item, err := a.Items.Create(r.Context(), me(r), in.Input, in.Publish)
	respond(w, http.StatusCreated, item, err)
}

func (a *api) updateItem(w http.ResponseWriter, r *http.Request) {
	var in items.Input
	if err := readJSON(r, &in); err != nil {
		writeError(w, err)
		return
	}
	item, err := a.Items.Update(r.Context(), me(r), r.PathValue("id"), in)
	respond(w, http.StatusOK, item, err)
}

func (a *api) publishItem(w http.ResponseWriter, r *http.Request) {
	item, err := a.Items.Publish(r.Context(), me(r), r.PathValue("id"))
	respond(w, http.StatusOK, item, err)
}

func (a *api) deleteItem(w http.ResponseWriter, r *http.Request) {
	respond(w, http.StatusNoContent, nil, a.Swaps.DeleteItem(r.Context(), me(r), r.PathValue("id")))
}

// uploadImage takes the raw JPEG as request body (Content-Type: image/jpeg).
func (a *api) uploadImage(w http.ResponseWriter, r *http.Request) {
	// Check ownership before writing anything to disk; AddImage re-checks
	// ownership and state under a row lock.
	item, err := a.Items.Get(r.Context(), me(r), r.PathValue("id"))
	if err == nil && item.OwnerID != me(r) {
		err = items.ErrNotFound
	}
	if err != nil {
		writeError(w, err)
		return
	}
	name, err := a.Storage.SaveJPEG(r.Body)
	if err != nil {
		writeError(w, err)
		return
	}
	image, err := a.Items.AddImage(r.Context(), me(r), r.PathValue("id"), name)
	if err != nil {
		_ = a.Storage.Delete(name)
	}
	respond(w, http.StatusCreated, image, err)
}

func (a *api) deleteImage(w http.ResponseWriter, r *http.Request) {
	name, err := a.Items.DeleteImage(r.Context(), me(r), r.PathValue("id"), r.PathValue("imageID"))
	if err == nil {
		err = a.Storage.Delete(name)
	}
	respond(w, http.StatusNoContent, nil, err)
}

func (a *api) serveImage(w http.ResponseWriter, r *http.Request) {
	path, ok := a.Storage.Path(r.PathValue("name"))
	if !ok {
		writeError(w, errNotFound)
		return
	}
	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	http.ServeFile(w, r, path)
}

// --- users, blocks, reports --------------------------------------------------

func (a *api) getUser(w http.ResponseWriter, r *http.Request) {
	profile, err := a.Users.Public(r.Context(), me(r), r.PathValue("id"))
	respond(w, http.StatusOK, profile, err)
}

func (a *api) block(w http.ResponseWriter, r *http.Request) {
	respond(w, http.StatusNoContent, nil, a.Users.Block(r.Context(), me(r), r.PathValue("id")))
}

func (a *api) unblock(w http.ResponseWriter, r *http.Request) {
	respond(w, http.StatusNoContent, nil, a.Users.Unblock(r.Context(), me(r), r.PathValue("id")))
}

func (a *api) report(w http.ResponseWriter, r *http.Request) {
	var in users.ReportInput
	if err := readJSON(r, &in); err != nil {
		writeError(w, err)
		return
	}
	respond(w, http.StatusNoContent, nil, a.Users.Report(r.Context(), me(r), in))
}

// --- offers ----------------------------------------------------------------

func (a *api) listOffers(w http.ResponseWriter, r *http.Request) {
	list, err := a.Swaps.ListOffers(r.Context(), me(r))
	respond(w, http.StatusOK, map[string]any{"offers": list}, err)
}

func (a *api) getOffer(w http.ResponseWriter, r *http.Request) {
	offer, err := a.Swaps.GetOffer(r.Context(), me(r), r.PathValue("id"))
	respond(w, http.StatusOK, offer, err)
}

func (a *api) createOffer(w http.ResponseWriter, r *http.Request) {
	var in struct {
		TargetItemID     string   `json:"target_item_id"`
		OfferedItemIDs   []string `json:"offered_item_ids"`
		RequestedItemIDs []string `json:"requested_item_ids"`
		Message          *string  `json:"message"`
	}
	if err := readJSON(r, &in); err != nil {
		writeError(w, err)
		return
	}
	if in.Message != nil && len([]rune(*in.Message)) > 1000 {
		writeError(w, apperr.New(apperr.Invalid, "invalid_input", "message must be at most 1000 characters"))
		return
	}
	offer, err := a.Swaps.CreateOffer(r.Context(), me(r), swaps.CreateOfferInput{
		TargetItemID:               in.TargetItemID,
		OfferedItemIDs:             in.OfferedItemIDs,
		AdditionalRequestedItemIDs: in.RequestedItemIDs,
		Message:                    in.Message,
	})
	respond(w, http.StatusCreated, offer, err)
}

func (a *api) offerAction(w http.ResponseWriter, r *http.Request) {
	ctx, userID, offerID := r.Context(), me(r), r.PathValue("id")
	var offer *swaps.Offer
	var err error
	switch r.PathValue("action") {
	case "accept":
		offer, err = a.Swaps.AcceptOffer(ctx, userID, offerID)
	case "decline":
		offer, err = a.Swaps.DeclineOffer(ctx, userID, offerID)
	case "cancel":
		offer, err = a.Swaps.CancelOffer(ctx, userID, offerID)
	case "confirm":
		offer, err = a.Swaps.ConfirmTrade(ctx, userID, offerID)
	default:
		err = errNotFound
	}
	respond(w, http.StatusOK, offer, err)
}

// --- notifications -----------------------------------------------------------

func (a *api) notifications(w http.ResponseWriter, r *http.Request) {
	list, err := a.Users.Notifications(r.Context(), me(r))
	respond(w, http.StatusOK, map[string]any{"notifications": list}, err)
}

func (a *api) readNotifications(w http.ResponseWriter, r *http.Request) {
	respond(w, http.StatusNoContent, nil, a.Users.MarkNotificationsRead(r.Context(), me(r)))
}
