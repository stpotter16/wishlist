package parse

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/stpotter16/wishlist/internal/types"
)

const maxFamilyPostBytes = 1 << 20

const maxAge = 130

// ParseFamilyPost decodes a new family from a JSON request body. It trims
// every text field and drops blank needs and wishes.
func ParseFamilyPost(w http.ResponseWriter, r *http.Request) (types.Family, error) {
	body := struct {
		Members []struct {
			Label                 string   `json:"label"`
			Age                   *int     `json:"age"`
			Gender                string   `json:"gender"`
			ShirtSize             string   `json:"shirtSize"`
			PantSize              string   `json:"pantSize"`
			ShoeSize              string   `json:"shoeSize"`
			PreferredBrandsColors string   `json:"preferredBrandsColors"`
			PreferredScents       string   `json:"preferredScents"`
			PrefersUnscented      bool     `json:"prefersUnscented"`
			PreferredGiftCards    string   `json:"preferredGiftCards"`
			AdditionalInfo        string   `json:"additionalInfo"`
			Needs                 []string `json:"needs"`
			Wishes                []string `json:"wishes"`
		} `json:"members"`
	}{}

	r.Body = http.MaxBytesReader(w, r.Body, maxFamilyPostBytes)
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return types.Family{}, errors.New("request body is not valid JSON")
	}

	if len(body.Members) == 0 {
		return types.Family{}, errors.New("a family needs at least one member")
	}

	family := types.Family{}
	for i, m := range body.Members {
		label := strings.TrimSpace(m.Label)
		if label == "" {
			return types.Family{}, fmt.Errorf("member %d needs a label", i+1)
		}
		if m.Age != nil && (*m.Age < 0 || *m.Age > maxAge) {
			return types.Family{}, fmt.Errorf("member %d has an invalid age", i+1)
		}

		family.Members = append(family.Members, types.FamilyMember{
			Label:                 label,
			Age:                   m.Age,
			Gender:                strings.TrimSpace(m.Gender),
			ShirtSize:             strings.TrimSpace(m.ShirtSize),
			PantSize:              strings.TrimSpace(m.PantSize),
			ShoeSize:              strings.TrimSpace(m.ShoeSize),
			PreferredBrandsColors: strings.TrimSpace(m.PreferredBrandsColors),
			PreferredScents:       strings.TrimSpace(m.PreferredScents),
			PrefersUnscented:      m.PrefersUnscented,
			PreferredGiftCards:    strings.TrimSpace(m.PreferredGiftCards),
			AdditionalInfo:        strings.TrimSpace(m.AdditionalInfo),
			Needs:                 parseWishlistItems(m.Needs),
			Wishes:                parseWishlistItems(m.Wishes),
		})
	}

	return family, nil
}

func parseWishlistItems(names []string) []types.WishlistItem {
	var items []types.WishlistItem
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		items = append(items, types.WishlistItem{Name: name})
	}
	return items
}
