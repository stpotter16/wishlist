package parse

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"unicode"

	"github.com/stpotter16/wishlist/internal/types"
)

const maxHouseholdPostBytes = 1 << 20

const maxAge = 130

// HouseholdForm is a household as the admin form submits it, before
// validation. It matches the JSON body of POST /admin/households.
type HouseholdForm struct {
	Kind    string                `json:"kind"`
	Members []HouseholdMemberForm `json:"members"`
}

type HouseholdMemberForm struct {
	FirstName             string   `json:"firstName"`
	LastInitial           string   `json:"lastInitial"`
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
}

// ParseHouseholdPost decodes a new household from a JSON request body and
// validates it with ParseHouseholdForm.
func ParseHouseholdPost(w http.ResponseWriter, r *http.Request) (types.Household, error) {
	var body HouseholdForm
	r.Body = http.MaxBytesReader(w, r.Body, maxHouseholdPostBytes)
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return types.Household{}, errors.New("request body is not valid JSON")
	}
	return ParseHouseholdForm(body)
}

// ParseHouseholdForm validates a household and builds its member labels. It
// trims every text field and drops blank needs and wishes. The first member is
// the head of the household. A family's head is labeled "Mom", and the rest of
// its members are children labeled "Child, <age>", or "Infant" at age 0. A
// single woman's household has only its head, labeled "Woman".
func ParseHouseholdForm(body HouseholdForm) (types.Household, error) {
	kind := types.HouseholdKind(body.Kind)
	headLabel := ""
	switch kind {
	case types.HouseholdKindFamily:
		if len(body.Members) < 2 {
			return types.Household{}, errors.New("a family needs a mom and at least one child")
		}
		headLabel = "Mom"
	case types.HouseholdKindSingle:
		if len(body.Members) != 1 {
			return types.Household{}, errors.New("a single woman's household has exactly one member")
		}
		headLabel = "Woman"
	default:
		return types.Household{}, errors.New("household must be a family or a single woman")
	}

	household := types.Household{Kind: kind}
	for i, m := range body.Members {
		firstName := strings.TrimSpace(m.FirstName)
		if firstName == "" {
			return types.Household{}, fmt.Errorf("member %d needs a first name", i+1)
		}

		// The head of the household shares her last initial but neither age
		// nor gender. Everyone after her is a child, named by first name only
		// and labeled by age.
		lastInitial := ""
		label := headLabel
		age := (*int)(nil)
		gender := ""
		if i == 0 {
			var err error
			lastInitial, err = parseLastInitial(m.LastInitial)
			if err != nil {
				return types.Household{}, err
			}
		} else {
			if m.Age == nil {
				return types.Household{}, fmt.Errorf("member %d needs an age", i+1)
			}
			if *m.Age < 0 || *m.Age > maxAge {
				return types.Household{}, fmt.Errorf("member %d has an invalid age", i+1)
			}
			label = childLabel(*m.Age)
			age = m.Age
			gender = strings.TrimSpace(m.Gender)
		}

		household.Members = append(household.Members, types.HouseholdMember{
			FirstName:             firstName,
			LastInitial:           lastInitial,
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

	return household, nil
}

// parseLastInitial accepts a single letter, optionally followed by a period,
// and returns it uppercased without the period.
func parseLastInitial(s string) (string, error) {
	initial := []rune(strings.TrimSuffix(strings.TrimSpace(s), "."))
	if len(initial) != 1 || !unicode.IsLetter(initial[0]) {
		return "", errors.New("the head of the household needs a one-letter last initial")
	}
	return strings.ToUpper(string(initial)), nil
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
