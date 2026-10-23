package sqlite_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/stpotter16/wishlist/internal/store"
	"github.com/stpotter16/wishlist/internal/types"
)

func TestCreateClaimMovesFamilyFromPublicToGiver(t *testing.T) {
	s, d := newTestStore(t)
	ctx := context.Background()

	if _, err := d.Exec(ctx, `
	INSERT INTO family (id, created_time, last_modified_time) VALUES
		(1, '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z');
	INSERT INTO family_member
		(family_id, label, created_time, last_modified_time)
	VALUES
		(1, 'Mom', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z');
	`); err != nil {
		t.Fatalf("seed: %v", err)
	}

	if err := s.CreateClaim(ctx, types.NewClaim{
		FamilyID: 1,
		Giver: types.Giver{
			Name:  "Giver",
			Email: "giver@example.com",
			Phone: "555-0100",
		},
		AccessTokenHash: "token-hash",
	}); err != nil {
		t.Fatalf("CreateClaim: %v", err)
	}

	family, err := s.GetClaimedFamily(ctx, "token-hash")
	if err != nil {
		t.Fatalf("GetClaimedFamily: %v", err)
	}
	if got, want := family.ID, 1; got != want {
		t.Errorf("family.ID=%d, want=%d", got, want)
	}
	if got, want := family.Members[0].Label, "Mom"; got != want {
		t.Errorf("member label=%q, want=%q", got, want)
	}

	if _, err := s.GetUnclaimedFamily(ctx, 1); err != store.ErrFamilyNotFound {
		t.Errorf("GetUnclaimedFamily err=%v, want=%v", err, store.ErrFamilyNotFound)
	}
}

func TestCreateClaimRejectsInvalidClaims(t *testing.T) {
	for _, tt := range []struct {
		explanation string
		familyID    int
		giverEmail  string
		errExpected error
	}{
		{
			"family that doesn't exist can't be claimed",
			99,
			"new@example.com",
			store.ErrFamilyNotFound,
		},
		{
			"family with no members can't be claimed",
			3,
			"new@example.com",
			store.ErrFamilyNotFound,
		},
		{
			"family that's already claimed can't be claimed again",
			2,
			"new@example.com",
			store.ErrFamilyAlreadyClaimed,
		},
		{
			"email that already claimed a family can't claim another",
			1,
			"giver@example.com",
			store.ErrEmailAlreadyClaimed,
		},
		{
			"email match ignores case",
			1,
			"GIVER@Example.com",
			store.ErrEmailAlreadyClaimed,
		},
	} {
		t.Run(fmt.Sprintf("%s [family=%d, email=%s]", tt.explanation, tt.familyID, tt.giverEmail), func(t *testing.T) {
			s, d := newTestStore(t)
			ctx := context.Background()

			// Family 1 is unclaimed. giver@example.com claimed family 2.
			// Family 3 has no members.
			if _, err := d.Exec(ctx, `
			INSERT INTO family (id, created_time, last_modified_time) VALUES
				(1, '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z'),
				(2, '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z'),
				(3, '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z');
			INSERT INTO family_member
				(family_id, label, created_time, last_modified_time)
			VALUES
				(1, 'Mom', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z'),
				(2, 'Dad', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z');
			INSERT INTO claim (
				family_id, giver_name, giver_email, giver_phone,
				access_token_hash, created_time
			) VALUES (
				2, 'Giver', 'giver@example.com', '555-0100', 'existing-hash',
				'2026-01-01T00:00:00Z'
			);
			`); err != nil {
				t.Fatalf("seed: %v", err)
			}

			err := s.CreateClaim(ctx, types.NewClaim{
				FamilyID: tt.familyID,
				Giver: types.Giver{
					Name:  "New Giver",
					Email: tt.giverEmail,
					Phone: "555-0199",
				},
				AccessTokenHash: "new-hash",
			})
			if got, want := err, tt.errExpected; got != want {
				t.Fatalf("err=%v, want=%v", got, want)
			}
		})
	}
}

func TestGetClaimedFamilyUnknownToken(t *testing.T) {
	s, _ := newTestStore(t)

	_, err := s.GetClaimedFamily(context.Background(), "unknown-hash")
	if got, want := err, store.ErrClaimNotFound; got != want {
		t.Errorf("err=%v, want=%v", got, want)
	}
}
