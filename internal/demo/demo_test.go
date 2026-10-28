package demo_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stpotter16/wishlist/internal/demo"
	"github.com/stpotter16/wishlist/internal/store/db"
	"github.com/stpotter16/wishlist/internal/store/sqlite"
	"github.com/stpotter16/wishlist/internal/types"
)

func newStore(t *testing.T) sqlite.Store {
	t.Helper()
	d, err := db.New(t.TempDir())
	if err != nil {
		t.Fatalf("db.New: %v", err)
	}
	s, err := sqlite.New(d)
	if err != nil {
		t.Fatalf("sqlite.New: %v", err)
	}
	return s
}

func TestSeedFillsEmptyDatabase(t *testing.T) {
	s := newStore(t)

	result, err := demo.Seed(context.Background(), s)
	if err != nil {
		t.Fatalf("Seed: %v", err)
	}
	if got, want := result, (demo.Result{Households: 12, Claims: 3}); got != want {
		t.Errorf("result=%+v, want=%+v", got, want)
	}

	households, err := s.ListHouseholds(context.Background())
	if err != nil {
		t.Fatalf("ListHouseholds: %v", err)
	}
	kinds := map[types.HouseholdKind]int{}
	claimed := 0
	for _, h := range households {
		kinds[h.Kind]++
		if h.IsClaimed {
			claimed++
			if !strings.HasSuffix(h.GiverEmail, "@example.com") {
				t.Errorf("household %d giver email=%q, want an example.com address", h.ID, h.GiverEmail)
			}
		}
	}
	if got, want := kinds[types.HouseholdKindFamily], 8; got != want {
		t.Errorf("families=%d, want=%d", got, want)
	}
	if got, want := kinds[types.HouseholdKindSingle], 4; got != want {
		t.Errorf("single women=%d, want=%d", got, want)
	}
	if got, want := claimed, 3; got != want {
		t.Errorf("claimed=%d, want=%d", got, want)
	}

	for _, summary := range households {
		household, err := s.GetHousehold(context.Background(), summary.ID)
		if err != nil {
			t.Fatalf("GetHousehold(%d): %v", summary.ID, err)
		}
		for _, m := range household.Members {
			if !strings.HasPrefix(m.FirstName, "Demo ") {
				t.Errorf("household %d member name=%q, want a Demo prefix", summary.ID, m.FirstName)
			}
		}
	}
}

func TestSeedRefusesDatabaseWithHouseholds(t *testing.T) {
	s := newStore(t)
	if _, err := s.CreateHousehold(context.Background(), types.Household{
		Kind:    types.HouseholdKindSingle,
		Members: []types.HouseholdMember{{FirstName: "Rosa", LastInitial: "M", Label: "Woman"}},
	}); err != nil {
		t.Fatalf("CreateHousehold: %v", err)
	}

	_, err := demo.Seed(context.Background(), s)
	if got, want := err, demo.ErrNotEmpty; !errors.Is(got, want) {
		t.Fatalf("err=%v, want=%v", got, want)
	}

	households, err := s.ListHouseholds(context.Background())
	if err != nil {
		t.Fatalf("ListHouseholds: %v", err)
	}
	if got, want := len(households), 1; got != want {
		t.Errorf("households=%d, want=%d", got, want)
	}
}
