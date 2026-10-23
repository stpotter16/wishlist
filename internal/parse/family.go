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

const momLabel = "Mom"

// ParseFamilyPost decodes a new family from a JSON request body. It trims
// every text field and drops blank needs and wishes. It labels members itself:
// the first is "Mom" and the rest are "Child, <age>", or "Infant" at age 0.
func ParseFamilyPost(w http.ResponseWriter, r *http.Request) (types.Family, error) {
	body := struct {
		Members []struct {
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
		// The first member is always Mom, who shares neither age nor gender.
		// Everyone after her is a child, labeled by age.
		label := momLabel
		age := (*int)(nil)
		gender := ""
		if i > 0 {
			if m.Age == nil {
				return types.Family{}, fmt.Errorf("member %d needs an age", i+1)
			}
			if *m.Age < 0 || *m.Age > maxAge {
				return types.Family{}, fmt.Errorf("member %d has an invalid age", i+1)
			}
			label = childLabel(*m.Age)
			age = m.Age
			gender = strings.TrimSpace(m.Gender)
		}

		family.Members = append(family.Members, types.FamilyMember{
			Label:                 label,
			Age:                   age,
			Gender:                gender,
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

func childLabel(age int) string {
	if age == 0 {
		return "Infant"
	}
	return fmt.Sprintf("Child, %d", age)
}
