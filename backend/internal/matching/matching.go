package matching

import (
	"math"
	"strings"

	"github.com/yasinatby/swappy/backend/internal/items"
	"github.com/yasinatby/swappy/backend/internal/models"
)

type Service struct {
	itemService *items.Service
}

func NewService(itemService *items.Service) *Service {
	return &Service{itemService: itemService}
}

func (s *Service) FindMatches(userID string) ([]models.SwapMatch, error) {
	// 1. Get all active items owned by current user
	myItems, err := s.itemService.List(items.Filter{
		UserID: userID,
		Status: models.ItemStatusAvailable,
		Limit:  100,
	}, userID)
	if err != nil {
		return nil, err
	}
	if len(myItems) == 0 {
		return []models.SwapMatch{}, nil
	}

	// 2. Get all available items on marketplace from other users
	allMarketItems, err := s.itemService.List(items.Filter{
		Status: models.ItemStatusAvailable,
		Limit:  200,
	}, userID)
	if err != nil {
		return nil, err
	}

	var matches []models.SwapMatch

	for _, myItem := range myItems {
		for _, theirItem := range allMarketItems {
			// Skip own items
			if theirItem.UserID == userID {
				continue
			}

			// Check matching:
			// Does their item match my trade interests?
			iWantTheirs := matchesInterest(theirItem, myItem.TradeInterests)
			// Does my item match their trade interests?
			theyWantMine := matchesInterest(myItem, theirItem.TradeInterests)

			if iWantTheirs && theyWantMine {
				// Perfect two-way barter match!
				diff := math.Abs(myItem.EstimatedValue - theirItem.EstimatedValue)
				score := 100
				if diff > 100 {
					score = 90
				}
				if diff > 300 {
					score = 80
				}

				otherOwner := models.UserPublicProfile{}
				if theirItem.Owner != nil {
					otherOwner = *theirItem.Owner
				}

				match := models.SwapMatch{
					MyItem:        myItem,
					TheirItem:     theirItem,
					OtherUser:     otherOwner,
					MatchScore:    score,
					MatchReason:   "🔥 Perfekter Tausch-Match: Du besitzt, was dein Partner sucht, und dein Partner besitzt, was du suchst!",
					EstimatedDiff: diff,
				}
				matches = append(matches, match)
			} else if iWantTheirs {
				// One-way interest match (good candidate)
				diff := math.Abs(myItem.EstimatedValue - theirItem.EstimatedValue)
				otherOwner := models.UserPublicProfile{}
				if theirItem.Owner != nil {
					otherOwner = *theirItem.Owner
				}

				match := models.SwapMatch{
					MyItem:        myItem,
					TheirItem:     theirItem,
					OtherUser:     otherOwner,
					MatchScore:    65,
					MatchReason:   "Dieses Objekt entspricht deinen Tauschinteressen.",
					EstimatedDiff: diff,
				}
				matches = append(matches, match)
			}
		}
	}

	return matches, nil
}

func matchesInterest(item models.Item, interests []string) bool {
	itemTitle := strings.ToLower(item.Title)
	itemCategory := strings.ToLower(item.Category)
	itemDesc := strings.ToLower(item.Description)

	for _, rawInterest := range interests {
		interest := strings.ToLower(strings.TrimSpace(rawInterest))
		if interest == "" {
			continue
		}

		// Exact or substring match in Title
		if strings.Contains(itemTitle, interest) || strings.Contains(interest, itemTitle) {
			return true
		}

		// Match category
		if strings.EqualFold(interest, itemCategory) {
			return true
		}

		// Word match in item description or title
		words := strings.Fields(interest)
		for _, w := range words {
			if len(w) >= 3 && (strings.Contains(itemTitle, w) || strings.Contains(itemDesc, w)) {
				return true
			}
		}
	}

	return false
}
