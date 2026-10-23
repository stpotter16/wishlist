package sqlite_test

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	"github.com/stpotter16/wishlist/internal/store"
	"github.com/stpotter16/wishlist/internal/types"
)

func TestListUnclaimedFamilies(t *testing.T) {
	s, d := newTestStore(t)
	ctx := context.Background()

	// Family 1 and 4 are unclaimed with members. Family 2 is claimed. Family 3
	// has no members yet.
	if _, err := d.Exec(ctx, `
	INSERT INTO family (id, created_time, last_modified_time) VALUES
		(1, '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z'),
		(2, '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z'),
		(3, '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z'),
		(4, '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z');
	INSERT INTO family_member
		(family_id, label, created_time, last_modified_time)
	VALUES
		(1, 'Mom', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z'),
		(1, 'Child, 7', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z'),
		(2, 'Dad', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z'),
		(4, 'Grandpa', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z');
	INSERT INTO claim (
		family_id, giver_name, giver_email, giver_phone, access_token_hash,
		created_time
	) VALUES (
		2, 'Giver', 'giver@example.com', '555-0100', 'dummy-hash',
		'2026-01-01T00:00:00Z'
	);
	`); err != nil {
		t.Fatalf("seed: %v", err)
	}

	families, err := s.ListUnclaimedFamilies(ctx)
	if err != nil {
		t.Fatalf("ListUnclaimedFamilies: %v", err)
	}

	want := []types.FamilySummary{
		{ID: 1, MemberLabels: []string{"Mom", "Child, 7"}},
		{ID: 4, MemberLabels: []string{"Grandpa"}},
	}
	if got := families; !reflect.DeepEqual(got, want) {
		t.Errorf("families=%+v, want=%+v", got, want)
	}
}

func TestListUnclaimedFamiliesEmpty(t *testing.T) {
	s, _ := newTestStore(t)

	families, err := s.ListUnclaimedFamilies(context.Background())
	if err != nil {
		t.Fatalf("ListUnclaimedFamilies: %v", err)
	}

	if got, want := len(families), 0; got != want {
		t.Errorf("len(families)=%d, want=%d", got, want)
	}
}

func TestGetUnclaimedFamily(t *testing.T) {
	s, d := newTestStore(t)
	ctx := context.Background()

	if _, err := d.Exec(ctx, `
	INSERT INTO family (id, created_time, last_modified_time) VALUES
		(1, '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z');
	INSERT INTO family_member (
		id, family_id, label, age, gender, shirt_size, pant_size, shoe_size,
		preferred_brands_colors, preferred_scents, prefers_unscented,
		preferred_gift_cards, additional_info, created_time, last_modified_time
	) VALUES (
		10, 1, 'Mom', NULL, 'F', 'L', '12', '8',
		'Blue', 'Lavender', 0,
		'Target', 'Loves gardening',
		'2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z'
	), (
		11, 1, 'Child, 7', 7, 'M', 'Youth M', '', '',
		'', '', 1,
		'', '',
		'2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z'
	);
	INSERT INTO wishlist_item (
		id, family_member_id, kind, name, notes, created_time,
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

	family, err := s.GetUnclaimedFamily(ctx, 1)
	if err != nil {
		t.Fatalf("GetUnclaimedFamily: %v", err)
	}

	childAge := 7
	want := types.Family{
		ID: 1,
		Members: []types.FamilyMember{
			{
				ID:                    10,
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
	if got := family; !reflect.DeepEqual(got, want) {
		t.Errorf("family=%+v, want=%+v", got, want)
	}
}

func TestGetUnclaimedFamilyNotFound(t *testing.T) {
	for _, tt := range []struct {
		explanation string
		familyID    int
	}{
		{
			"family that doesn't exist is not found",
			99,
		},
		{
			"claimed family is not found",
			2,
		},
		{
			"family with no members is not found",
			3,
		},
	} {
		t.Run(fmt.Sprintf("%s [%d]", tt.explanation, tt.familyID), func(t *testing.T) {
			s, d := newTestStore(t)
			ctx := context.Background()

			// Family 2 is claimed. Family 3 has no members.
			if _, err := d.Exec(ctx, `
			INSERT INTO family (id, created_time, last_modified_time) VALUES
				(2, '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z'),
				(3, '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z');
			INSERT INTO family_member
				(family_id, label, created_time, last_modified_time)
			VALUES
				(2, 'Dad', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z');
			INSERT INTO claim (
				family_id, giver_name, giver_email, giver_phone,
				access_token_hash, created_time
			) VALUES (
				2, 'Giver', 'giver@example.com', '555-0100', 'dummy-hash',
				'2026-01-01T00:00:00Z'
			);
			`); err != nil {
				t.Fatalf("seed: %v", err)
			}

			_, err := s.GetUnclaimedFamily(ctx, tt.familyID)
			if got, want := err, store.ErrFamilyNotFound; got != want {
				t.Errorf("err=%v, want=%v", got, want)
			}
		})
	}
}

func TestCreateFamily(t *testing.T) {
	s, _ := newTestStore(t)
	ctx := context.Background()

	childAge := 7
	id, err := s.CreateFamily(ctx, types.Family{
		Members: []types.FamilyMember{
			{
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
		t.Fatalf("CreateFamily: %v", err)
	}

	family, err := s.GetUnclaimedFamily(ctx, id)
	if err != nil {
		t.Fatalf("GetUnclaimedFamily: %v", err)
	}

	// On a fresh database, SQLite assigns member and item IDs starting at 1 in
	// insertion order.
	want := types.Family{
		ID: id,
		Members: []types.FamilyMember{
			{
				ID:                    1,
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
	if got := family; !reflect.DeepEqual(got, want) {
		t.Errorf("family=%+v, want=%+v", got, want)
	}
}

func TestListFamilies(t *testing.T) {
	s, d := newTestStore(t)
	ctx := context.Background()

	// Family 1 is unclaimed. Family 2 is claimed. Family 3 has no members.
	if _, err := d.Exec(ctx, `
	INSERT INTO family (id, created_time, last_modified_time) VALUES
		(1, '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z'),
		(2, '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z'),
		(3, '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z');
	INSERT INTO family_member
		(family_id, label, created_time, last_modified_time)
	VALUES
		(1, 'Mom', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z'),
		(1, 'Child, 7', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z'),
		(2, 'Dad', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z');
	INSERT INTO claim (
		family_id, giver_name, giver_email, giver_phone, access_token_hash,
		created_time
	) VALUES (
		2, 'Giver', 'giver@example.com', '555-0100', 'dummy-hash',
		'2026-01-01T00:00:00Z'
	);
	`); err != nil {
		t.Fatalf("seed: %v", err)
	}

	families, err := s.ListFamilies(ctx)
	if err != nil {
		t.Fatalf("ListFamilies: %v", err)
	}

	want := []types.FamilySummary{
		{ID: 1, MemberLabels: []string{"Mom", "Child, 7"}, IsClaimed: false},
		{ID: 2, MemberLabels: []string{"Dad"}, IsClaimed: true},
		{ID: 3, MemberLabels: nil, IsClaimed: false},
	}
	if got := families; !reflect.DeepEqual(got, want) {
		t.Errorf("families=%+v, want=%+v", got, want)
	}
}

func TestUpdateFamilyReplacesMembersAndKeepsClaim(t *testing.T) {
	s, d := newTestStore(t)
	ctx := context.Background()

	// Family 1 has two members and a claim.
	if _, err := d.Exec(ctx, `
	INSERT INTO family (id, created_time, last_modified_time) VALUES
		(1, '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z');
	INSERT INTO family_member
		(id, family_id, label, created_time, last_modified_time)
	VALUES
		(10, 1, 'Mom', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z'),
		(11, 1, 'Child, 7', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z');
	INSERT INTO wishlist_item (
		family_member_id, kind, name, created_time, last_modified_time
	) VALUES
		(10, 'need', 'Old coat', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z');
	INSERT INTO claim (
		family_id, giver_name, giver_email, giver_phone, access_token_hash,
		created_time
	) VALUES (
		1, 'Giver', 'giver@example.com', '555-0100', 'token-hash',
		'2026-01-01T00:00:00Z'
	);
	`); err != nil {
		t.Fatalf("seed: %v", err)
	}

	if err := s.UpdateFamily(ctx, 1, types.Family{
		Members: []types.FamilyMember{
			{
				Label:  "Mom",
				Needs:  []types.WishlistItem{{Name: "Winter coat"}},
				Wishes: []types.WishlistItem{{Name: "Book"}},
			},
		},
	}); err != nil {
		t.Fatalf("UpdateFamily: %v", err)
	}

	// Members and items get new IDs, so compare only what the caller supplied.
	family, err := s.GetClaimedFamily(ctx, "token-hash")
	if err != nil {
		t.Fatalf("GetClaimedFamily: %v", err)
	}
	if got, want := len(family.Members), 1; got != want {
		t.Fatalf("len(Members)=%d, want=%d", got, want)
	}
	if got, want := family.Members[0].Label, "Mom"; got != want {
		t.Errorf("Label=%q, want=%q", got, want)
	}
	if got, want := len(family.Members[0].Needs), 1; got != want {
		t.Fatalf("len(Needs)=%d, want=%d", got, want)
	}
	if got, want := family.Members[0].Needs[0].Name, "Winter coat"; got != want {
		t.Errorf("Needs[0].Name=%q, want=%q", got, want)
	}
	if got, want := len(family.Members[0].Wishes), 1; got != want {
		t.Fatalf("len(Wishes)=%d, want=%d", got, want)
	}
	if got, want := family.Members[0].Wishes[0].Name, "Book"; got != want {
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

func TestUpdateFamilyNotFound(t *testing.T) {
	s, _ := newTestStore(t)

	err := s.UpdateFamily(context.Background(), 99, types.Family{
		Members: []types.FamilyMember{{Label: "Mom"}},
	})
	if got, want := err, store.ErrFamilyNotFound; got != want {
		t.Errorf("err=%v, want=%v", got, want)
	}
}

func TestDeleteFamily(t *testing.T) {
	for _, tt := range []struct {
		explanation    string
		familyID       int
		errExpected    error
		isGoneExpected bool
	}{
		{
			"unclaimed family is deleted",
			1,
			nil,
			true,
		},
		{
			"claimed family is kept",
			2,
			store.ErrFamilyAlreadyClaimed,
			false,
		},
		{
			"family that doesn't exist is not found",
			99,
			store.ErrFamilyNotFound,
			true,
		},
	} {
		t.Run(fmt.Sprintf("%s [%d]", tt.explanation, tt.familyID), func(t *testing.T) {
			s, d := newTestStore(t)
			ctx := context.Background()

			// Family 1 is unclaimed. Family 2 is claimed.
			if _, err := d.Exec(ctx, `
			INSERT INTO family (id, created_time, last_modified_time) VALUES
				(1, '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z'),
				(2, '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z');
			INSERT INTO family_member
				(id, family_id, label, created_time, last_modified_time)
			VALUES
				(10, 1, 'Mom', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z'),
				(20, 2, 'Dad', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z');
			INSERT INTO claim (
				family_id, giver_name, giver_email, giver_phone,
				access_token_hash, created_time
			) VALUES (
				2, 'Giver', 'giver@example.com', '555-0100', 'token-hash',
				'2026-01-01T00:00:00Z'
			);
			`); err != nil {
				t.Fatalf("seed: %v", err)
			}

			err := s.DeleteFamily(ctx, tt.familyID)
			if got, want := err, tt.errExpected; got != want {
				t.Fatalf("err=%v, want=%v", got, want)
			}

			_, err = s.GetFamily(ctx, tt.familyID)
			if got, want := err == store.ErrFamilyNotFound, tt.isGoneExpected; got != want {
				t.Errorf("family gone=%v, want=%v (GetFamily err=%v)", got, want, err)
			}
		})
	}
}

func TestDeletedFamilyIDIsNeverReused(t *testing.T) {
	s, _ := newTestStore(t)
	ctx := context.Background()

	// The family ID is the public "Family N" label, so a new family must
	// never take over a deleted family's label.
	newFamily := types.Family{Members: []types.FamilyMember{{Label: "Mom"}}}
	deletedID, err := s.CreateFamily(ctx, newFamily)
	if err != nil {
		t.Fatalf("CreateFamily: %v", err)
	}
	if err := s.DeleteFamily(ctx, deletedID); err != nil {
		t.Fatalf("DeleteFamily: %v", err)
	}

	nextID, err := s.CreateFamily(ctx, newFamily)
	if err != nil {
		t.Fatalf("CreateFamily: %v", err)
	}
	if nextID == deletedID {
		t.Errorf("new family reused deleted family ID %d", deletedID)
	}
}
