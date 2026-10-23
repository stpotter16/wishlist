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
