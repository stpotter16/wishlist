package sqlite_test

import (
	"context"
	"reflect"
	"testing"

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
