// Package demo seeds an empty database with made-up households, so
// stakeholders can see the app with a full drive's worth of data.
package demo

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/stpotter16/wishlist/internal/auth"
	"github.com/stpotter16/wishlist/internal/parse"
	"github.com/stpotter16/wishlist/internal/types"
)

// households.json uses the admin form's JSON shape, so ParseHouseholdForm
// applies the same rules as admin-entered households. Every name starts with
// "Demo" so no one mistakes a demo household for a real one.
//
//go:embed households.json
var householdsJSON []byte

// demoGivers claim households by their position in households.json. Their
// emails use example.com, which can't receive mail.
var demoGivers = map[int]types.Giver{
	1: {Name: "Demo Giver One", Email: "giver1@example.com", Phone: "555-0101"},
	3: {Name: "Demo Giver Two", Email: "giver2@example.com", Phone: "555-0102"},
	9: {Name: "Demo Giver Three", Email: "giver3@example.com", Phone: "555-0103"},
}

var ErrNotEmpty = errors.New("database already has households")

type Store interface {
	ListHouseholds(ctx context.Context) ([]types.HouseholdSummary, error)
	CreateHousehold(ctx context.Context, household types.Household) (int, error)
	CreateClaim(ctx context.Context, claim types.NewClaim) error
}

type Result struct {
	Households int
	Claims     int
}

// Seed adds the demo households and claims. It refuses to touch a database
// that already has households, so demo data never mixes with real data.
// Claims go straight to the store, so no giver is emailed and no private link
// is shown. Givers' links are discarded.
func Seed(ctx context.Context, s Store) (Result, error) {
	existing, err := s.ListHouseholds(ctx)
	if err != nil {
		return Result{}, err
	}
	if len(existing) > 0 {
		return Result{}, ErrNotEmpty
	}

	var forms []parse.HouseholdForm
	if err := json.Unmarshal(householdsJSON, &forms); err != nil {
		return Result{}, fmt.Errorf("demo households are not valid JSON: %w", err)
	}

	var result Result
	for i, form := range forms {
		household, err := parse.ParseHouseholdForm(form)
		if err != nil {
			return result, fmt.Errorf("demo household %d is invalid: %w", i, err)
		}
		id, err := s.CreateHousehold(ctx, household)
		if err != nil {
			return result, err
		}
		result.Households++

		giver, ok := demoGivers[i]
		if !ok {
			continue
		}
		_, tokenHash, err := auth.NewAccessToken()
		if err != nil {
			return result, err
		}
		if err := s.CreateClaim(ctx, types.NewClaim{
			HouseholdID:     id,
			Giver:           giver,
			AccessTokenHash: tokenHash,
		}); err != nil {
			return result, err
		}
		result.Claims++
	}
	return result, nil
}
