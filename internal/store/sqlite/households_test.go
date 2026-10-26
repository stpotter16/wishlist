package sqlite_test

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	"github.com/stpotter16/wishlist/internal/store"
	"github.com/stpotter16/wishlist/internal/types"
)

func TestListUnclaimedHouseholds(t *testing.T) {
	s, d := newTestStore(t)
	ctx := context.Background()

	// Household 1 and 4 are unclaimed with members. Household 2 is claimed.
	// Household 3 has no members yet.
	if _, err := d.Exec(ctx, `
	INSERT INTO household (id, kind, created_time, last_modified_time) VALUES
		(1, 'family', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z'),
		(2, 'family', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z'),
		(3, 'family', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z'),
		(4, 'single', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z');
	INSERT INTO household_member
		(household_id, first_name, label, created_time, last_modified_time)
	VALUES
		(1, 'Maria', 'Mom', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z'),
		(1, 'Sofia', 'Child, 7', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z'),
		(2, 'Dan', 'Dad', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z'),
		(4, 'Rosa', 'Woman', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z');
	INSERT INTO claim (
		household_id, giver_name, giver_email, giver_phone, access_token_hash,
		created_time
	) VALUES (
		2, 'Giver', 'giver@example.com', '555-0100', 'dummy-hash',
		'2026-01-01T00:00:00Z'
	);
	`); err != nil {
		t.Fatalf("seed: %v", err)
	}

	households, err := s.ListUnclaimedHouseholds(ctx)
	if err != nil {
		t.Fatalf("ListUnclaimedHouseholds: %v", err)
	}

	want := []types.HouseholdSummary{
		{ID: 1, Kind: types.HouseholdKindFamily, MemberLabels: []string{"Mom", "Child, 7"}},
		{ID: 4, Kind: types.HouseholdKindSingle, MemberLabels: []string{"Woman"}},
	}
	if got := households; !reflect.DeepEqual(got, want) {
		t.Errorf("households=%+v, want=%+v", got, want)
	}
}

func TestListUnclaimedHouseholdsEmpty(t *testing.T) {
	s, _ := newTestStore(t)

	households, err := s.ListUnclaimedHouseholds(context.Background())
	if err != nil {
		t.Fatalf("ListUnclaimedHouseholds: %v", err)
	}

	if got, want := len(households), 0; got != want {
		t.Errorf("len(households)=%d, want=%d", got, want)
	}
}

func TestGetUnclaimedHousehold(t *testing.T) {
	s, d := newTestStore(t)
	ctx := context.Background()

	if _, err := d.Exec(ctx, `
	INSERT INTO household (id, kind, created_time, last_modified_time) VALUES
		(1, 'family', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z');
	INSERT INTO household_member (
		id, household_id, first_name, last_initial, label, age, gender, shirt_size,
		pant_size, shoe_size, preferred_brands_colors, preferred_scents, prefers_unscented,
		preferred_gift_cards, additional_info, created_time, last_modified_time
	) VALUES (
		10, 1, 'Maria', 'G', 'Mom', NULL, 'F', 'L', '12', '8',
		'Blue', 'Lavender', 0,
		'Target', 'Loves gardening',
		'2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z'
	), (
		11, 1, 'Sofia', '', 'Child, 7', 7, 'M', 'Youth M', '', '',
		'', '', 1,
		'', '',
		'2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z'
	);
	INSERT INTO wishlist_item (
		id, household_member_id, kind, name, notes, created_time,
		last_modified_time
	) VALUES
		(100, 10, 'need', 'Winter coat', 'Size L', '2026-01-01T00:00:00Z',
			'2026-01-01T00:00:00Z'),
		(101, 11, 'wish', 'LEGO set', '', '2026-01-01T00:00:00Z',
			'2026-01-01T00:00:00Z'),
		(102, 11, 'need', 'Rain boots', '', '2026-01-01T00:00:00Z',
			'2026-01-01T00:00:00Z');
	`); err != nil {
		t.Fatalf("seed: %v", err)
	}

	household, err := s.GetUnclaimedHousehold(ctx, 1)
	if err != nil {
		t.Fatalf("GetUnclaimedHousehold: %v", err)
	}

	childAge := 7
	want := types.Household{
		ID:   1,
		Kind: types.HouseholdKindFamily,
		Members: []types.HouseholdMember{
			{
				ID:                    10,
				FirstName:             "Maria",
				LastInitial:           "G",
				Label:                 "Mom",
				Age:                   nil,
				Gender:                "F",
				ShirtSize:             "L",
				PantSize:              "12",
				ShoeSize:              "8",
				PreferredBrandsColors: "Blue",
				PreferredScents:       "Lavender",
				PrefersUnscented:      false,
				PreferredGiftCards:    "Target",
				AdditionalInfo:        "Loves gardening",
				Needs: []types.WishlistItem{
					{ID: 100, Name: "Winter coat", Notes: "Size L"},
				},
				Wishes: nil,
			},
			{
				ID:               11,
				FirstName:        "Sofia",
				Label:            "Child, 7",
				Age:              &childAge,
				Gender:           "M",
				ShirtSize:        "Youth M",
				PrefersUnscented: true,
				Needs: []types.WishlistItem{
					{ID: 102, Name: "Rain boots"},
				},
				Wishes: []types.WishlistItem{
					{ID: 101, Name: "LEGO set"},
				},
			},
		},
	}
	if got := household; !reflect.DeepEqual(got, want) {
		t.Errorf("household=%+v, want=%+v", got, want)
	}
}

func TestGetUnclaimedHouseholdNotFound(t *testing.T) {
	for _, tt := range []struct {
		explanation string
		householdID int
	}{
		{
			"household that doesn't exist is not found",
			99,
		},
		{
			"claimed household is not found",
			2,
		},
		{
			"household with no members is not found",
			3,
		},
	} {
		t.Run(fmt.Sprintf("%s [%d]", tt.explanation, tt.householdID), func(t *testing.T) {
			s, d := newTestStore(t)
			ctx := context.Background()

			// Household 2 is claimed. Household 3 has no members.
			if _, err := d.Exec(ctx, `
			INSERT INTO household (id, kind, created_time, last_modified_time) VALUES
				(2, 'family', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z'),
				(3, 'family', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z');
			INSERT INTO household_member
				(household_id, first_name, label, created_time, last_modified_time)
			VALUES
				(2, 'Dan', 'Dad', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z');
			INSERT INTO claim (
				household_id, giver_name, giver_email, giver_phone,
				access_token_hash, created_time
			) VALUES (
				2, 'Giver', 'giver@example.com', '555-0100', 'dummy-hash',
				'2026-01-01T00:00:00Z'
			);
			`); err != nil {
				t.Fatalf("seed: %v", err)
			}

			_, err := s.GetUnclaimedHousehold(ctx, tt.householdID)
			if got, want := err, store.ErrHouseholdNotFound; got != want {
				t.Errorf("err=%v, want=%v", got, want)
			}
		})
	}
}

func TestCreateHousehold(t *testing.T) {
	s, _ := newTestStore(t)
	ctx := context.Background()

	childAge := 7
	id, err := s.CreateHousehold(ctx, types.Household{
		Kind: types.HouseholdKindFamily,
		Members: []types.HouseholdMember{
			{
				FirstName:             "Maria",
				LastInitial:           "G",
				Label:                 "Mom",
				Gender:                "F",
				ShirtSize:             "L",
				PantSize:              "12",
				ShoeSize:              "8",
				PreferredBrandsColors: "Blue",
				PreferredScents:       "Lavender",
				PreferredGiftCards:    "Target",
				AdditionalInfo:        "Loves gardening",
				Needs: []types.WishlistItem{
					{Name: "Winter coat"},
				},
			},
			{
				FirstName:        "Sofia",
				Label:            "Child, 7",
				Age:              &childAge,
				PrefersUnscented: true,
				Needs: []types.WishlistItem{
					{Name: "Rain boots"},
				},
				Wishes: []types.WishlistItem{
					{Name: "LEGO set"},
					{Name: "Art supplies"},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("CreateHousehold: %v", err)
	}

	household, err := s.GetUnclaimedHousehold(ctx, id)
	if err != nil {
		t.Fatalf("GetUnclaimedHousehold: %v", err)
	}

	// On a fresh database, SQLite assigns member and item IDs starting at 1 in
	// insertion order.
	want := types.Household{
		ID:   id,
		Kind: types.HouseholdKindFamily,
		Members: []types.HouseholdMember{
			{
				ID:                    1,
				FirstName:             "Maria",
				LastInitial:           "G",
				Label:                 "Mom",
				Gender:                "F",
				ShirtSize:             "L",
				PantSize:              "12",
				ShoeSize:              "8",
				PreferredBrandsColors: "Blue",
				PreferredScents:       "Lavender",
				PreferredGiftCards:    "Target",
				AdditionalInfo:        "Loves gardening",
				Needs: []types.WishlistItem{
					{ID: 1, Name: "Winter coat"},
				},
			},
			{
				ID:               2,
				FirstName:        "Sofia",
				Label:            "Child, 7",
				Age:              &childAge,
				PrefersUnscented: true,
				Needs: []types.WishlistItem{
					{ID: 2, Name: "Rain boots"},
				},
				Wishes: []types.WishlistItem{
					{ID: 3, Name: "LEGO set"},
					{ID: 4, Name: "Art supplies"},
				},
			},
		},
	}
	if got := household; !reflect.DeepEqual(got, want) {
		t.Errorf("household=%+v, want=%+v", got, want)
	}
}

func TestListHouseholds(t *testing.T) {
	s, d := newTestStore(t)
	ctx := context.Background()

	// Household 1 is unclaimed. Household 2 is claimed. Household 3 has no members.
	if _, err := d.Exec(ctx, `
	INSERT INTO household (id, kind, created_time, last_modified_time) VALUES
		(1, 'family', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z'),
		(2, 'single', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z'),
		(3, 'family', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z');
	INSERT INTO household_member
		(household_id, first_name, label, created_time, last_modified_time)
	VALUES
		(1, 'Maria', 'Mom', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z'),
		(1, 'Sofia', 'Child, 7', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z'),
		(2, 'Rosa', 'Woman', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z');
	INSERT INTO claim (
		household_id, giver_name, giver_email, giver_phone, access_token_hash,
		created_time
	) VALUES (
		2, 'Giver', 'giver@example.com', '555-0100', 'dummy-hash',
		'2026-01-01T00:00:00Z'
	);
	`); err != nil {
		t.Fatalf("seed: %v", err)
	}

	households, err := s.ListHouseholds(ctx)
	if err != nil {
		t.Fatalf("ListHouseholds: %v", err)
	}

	want := []types.HouseholdSummary{
		{ID: 1, Kind: types.HouseholdKindFamily, MemberLabels: []string{"Mom", "Child, 7"}, IsClaimed: false},
		{ID: 2, Kind: types.HouseholdKindSingle, MemberLabels: []string{"Woman"}, IsClaimed: true, GiverName: "Giver", GiverEmail: "giver@example.com"},
		{ID: 3, Kind: types.HouseholdKindFamily, MemberLabels: nil, IsClaimed: false},
	}
	if got := households; !reflect.DeepEqual(got, want) {
		t.Errorf("households=%+v, want=%+v", got, want)
	}
}

func TestUpdateHouseholdReplacesKindAndMembersAndKeepsClaim(t *testing.T) {
	s, d := newTestStore(t)
	ctx := context.Background()

	// Household 1 is a family with two members and a claim.
	if _, err := d.Exec(ctx, `
	INSERT INTO household (id, kind, created_time, last_modified_time) VALUES
		(1, 'family', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z');
	INSERT INTO household_member
		(id, household_id, first_name, label, created_time, last_modified_time)
	VALUES
		(10, 1, 'Maria', 'Mom', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z'),
		(11, 1, 'Sofia', 'Child, 7', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z');
	INSERT INTO wishlist_item (
		household_member_id, kind, name, created_time, last_modified_time
	) VALUES
		(10, 'need', 'Old coat', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z');
	INSERT INTO claim (
		household_id, giver_name, giver_email, giver_phone, access_token_hash,
		created_time
	) VALUES (
		1, 'Giver', 'giver@example.com', '555-0100', 'token-hash',
		'2026-01-01T00:00:00Z'
	);
	`); err != nil {
		t.Fatalf("seed: %v", err)
	}

	if err := s.UpdateHousehold(ctx, 1, types.Household{
		Kind: types.HouseholdKindSingle,
		Members: []types.HouseholdMember{
			{
				FirstName:   "Rosa",
				LastInitial: "M",
				Label:       "Woman",
				Needs:       []types.WishlistItem{{Name: "Winter coat"}},
				Wishes:      []types.WishlistItem{{Name: "Book"}},
			},
		},
	}); err != nil {
		t.Fatalf("UpdateHousehold: %v", err)
	}

	// Members and items get new IDs, so compare only what the caller supplied.
	household, err := s.GetClaimedHousehold(ctx, "token-hash")
	if err != nil {
		t.Fatalf("GetClaimedHousehold: %v", err)
	}
	if got, want := household.Kind, types.HouseholdKindSingle; got != want {
		t.Errorf("Kind=%q, want=%q", got, want)
	}
	if got, want := len(household.Members), 1; got != want {
		t.Fatalf("len(Members)=%d, want=%d", got, want)
	}
	if got, want := household.Members[0].Name(), "Rosa M."; got != want {
		t.Errorf("Name()=%q, want=%q", got, want)
	}
	if got, want := household.Members[0].Label, "Woman"; got != want {
		t.Errorf("Label=%q, want=%q", got, want)
	}
	if got, want := len(household.Members[0].Needs), 1; got != want {
		t.Fatalf("len(Needs)=%d, want=%d", got, want)
	}
	if got, want := household.Members[0].Needs[0].Name, "Winter coat"; got != want {
		t.Errorf("Needs[0].Name=%q, want=%q", got, want)
	}
	if got, want := len(household.Members[0].Wishes), 1; got != want {
		t.Fatalf("len(Wishes)=%d, want=%d", got, want)
	}
	if got, want := household.Members[0].Wishes[0].Name, "Book"; got != want {
		t.Errorf("Wishes[0].Name=%q, want=%q", got, want)
	}

	var itemCount int
	if err := d.QueryRow(ctx, `SELECT COUNT(*) FROM wishlist_item`).Scan(&itemCount); err != nil {
		t.Fatalf("count items: %v", err)
	}
	if got, want := itemCount, 2; got != want {
		t.Errorf("wishlist items=%d, want=%d", got, want)
	}
}

func TestUpdateHouseholdNotFound(t *testing.T) {
	s, _ := newTestStore(t)

	err := s.UpdateHousehold(context.Background(), 99, types.Household{
		Kind:    types.HouseholdKindSingle,
		Members: []types.HouseholdMember{{FirstName: "Rosa", LastInitial: "M", Label: "Woman"}},
	})
	if got, want := err, store.ErrHouseholdNotFound; got != want {
		t.Errorf("err=%v, want=%v", got, want)
	}
}

func TestDeleteHousehold(t *testing.T) {
	for _, tt := range []struct {
		explanation    string
		householdID    int
		errExpected    error
		isGoneExpected bool
	}{
		{
			"unclaimed household is deleted",
			1,
			nil,
			true,
		},
		{
			"claimed household is kept",
			2,
			store.ErrHouseholdAlreadyClaimed,
			false,
		},
		{
			"household that doesn't exist is not found",
			99,
			store.ErrHouseholdNotFound,
			true,
		},
	} {
		t.Run(fmt.Sprintf("%s [%d]", tt.explanation, tt.householdID), func(t *testing.T) {
			s, d := newTestStore(t)
			ctx := context.Background()

			// Household 1 is unclaimed. Household 2 is claimed.
			if _, err := d.Exec(ctx, `
			INSERT INTO household (id, kind, created_time, last_modified_time) VALUES
				(1, 'family', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z'),
				(2, 'family', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z');
			INSERT INTO household_member
				(id, household_id, first_name, label, created_time, last_modified_time)
			VALUES
				(10, 1, 'Maria', 'Mom', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z'),
				(20, 2, 'Dan', 'Dad', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z');
			INSERT INTO claim (
				household_id, giver_name, giver_email, giver_phone,
				access_token_hash, created_time
			) VALUES (
				2, 'Giver', 'giver@example.com', '555-0100', 'token-hash',
				'2026-01-01T00:00:00Z'
			);
			`); err != nil {
				t.Fatalf("seed: %v", err)
			}

			err := s.DeleteHousehold(ctx, tt.householdID)
			if got, want := err, tt.errExpected; got != want {
				t.Fatalf("err=%v, want=%v", got, want)
			}

			_, err = s.GetHousehold(ctx, tt.householdID)
			if got, want := err == store.ErrHouseholdNotFound, tt.isGoneExpected; got != want {
				t.Errorf("household gone=%v, want=%v (GetHousehold err=%v)", got, want, err)
			}
		})
	}
}

func TestDeletedHouseholdIDIsNeverReused(t *testing.T) {
	s, _ := newTestStore(t)
	ctx := context.Background()

	// The household ID is the public "Household N" label, so a new household must
	// never take over a deleted household's label.
	newHousehold := types.Household{
		Kind:    types.HouseholdKindSingle,
		Members: []types.HouseholdMember{{FirstName: "Rosa", LastInitial: "M", Label: "Woman"}},
	}
	deletedID, err := s.CreateHousehold(ctx, newHousehold)
	if err != nil {
		t.Fatalf("CreateHousehold: %v", err)
	}
	if err := s.DeleteHousehold(ctx, deletedID); err != nil {
		t.Fatalf("DeleteHousehold: %v", err)
	}

	nextID, err := s.CreateHousehold(ctx, newHousehold)
	if err != nil {
		t.Fatalf("CreateHousehold: %v", err)
	}
	if nextID == deletedID {
		t.Errorf("new household reused deleted household ID %d", deletedID)
	}
}
