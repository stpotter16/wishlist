package sqlite_test

import (
	"context"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/stpotter16/wishlist/internal/store"
	"github.com/stpotter16/wishlist/internal/types"
)

func TestCreateClaimMovesHouseholdFromPublicToGiver(t *testing.T) {
	s, d := newTestStore(t)
	ctx := context.Background()

	if _, err := d.Exec(ctx, `
	INSERT INTO household (id, kind, created_time, last_modified_time) VALUES
		(1, 'family', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z');
	INSERT INTO household_member
		(household_id, first_name, label, created_time, last_modified_time)
	VALUES
		(1, 'Maria', 'Mom', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z');
	`); err != nil {
		t.Fatalf("seed: %v", err)
	}

	if err := s.CreateClaim(ctx, types.NewClaim{
		HouseholdID: 1,
		Giver: types.Giver{
			Name:  "Giver",
			Email: "giver@example.com",
			Phone: "555-0100",
		},
		AccessTokenHash: "token-hash",
	}); err != nil {
		t.Fatalf("CreateClaim: %v", err)
	}

	household, err := s.GetClaimedHousehold(ctx, "token-hash")
	if err != nil {
		t.Fatalf("GetClaimedHousehold: %v", err)
	}
	if got, want := household.ID, 1; got != want {
		t.Errorf("household.ID=%d, want=%d", got, want)
	}
	if got, want := household.Members[0].Label, "Mom"; got != want {
		t.Errorf("member label=%q, want=%q", got, want)
	}

	if _, err := s.GetUnclaimedHousehold(ctx, 1); err != store.ErrHouseholdNotFound {
		t.Errorf("GetUnclaimedHousehold err=%v, want=%v", err, store.ErrHouseholdNotFound)
	}
}

func TestCreateClaimRejectsInvalidClaims(t *testing.T) {
	for _, tt := range []struct {
		explanation string
		householdID int
		giverEmail  string
		errExpected error
	}{
		{
			"household that doesn't exist can't be claimed",
			99,
			"new@example.com",
			store.ErrHouseholdNotFound,
		},
		{
			"household with no members can't be claimed",
			3,
			"new@example.com",
			store.ErrHouseholdNotFound,
		},
		{
			"household that's already claimed can't be claimed again",
			2,
			"new@example.com",
			store.ErrHouseholdAlreadyClaimed,
		},
		{
			"email that already claimed a household can't claim another",
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
		t.Run(fmt.Sprintf("%s [household=%d, email=%s]", tt.explanation, tt.householdID, tt.giverEmail), func(t *testing.T) {
			s, d := newTestStore(t)
			ctx := context.Background()

			// Household 1 is unclaimed. giver@example.com claimed household 2.
			// Household 3 has no members.
			if _, err := d.Exec(ctx, `
			INSERT INTO household (id, kind, created_time, last_modified_time) VALUES
				(1, 'family', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z'),
				(2, 'family', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z'),
				(3, 'family', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z');
			INSERT INTO household_member
				(household_id, first_name, label, created_time, last_modified_time)
			VALUES
				(1, 'Maria', 'Mom', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z'),
				(2, 'Dan', 'Dad', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z');
			INSERT INTO claim (
				household_id, giver_name, giver_email, giver_phone,
				access_token_hash, created_time
			) VALUES (
				2, 'Giver', 'giver@example.com', '555-0100', 'existing-hash',
				'2026-01-01T00:00:00Z'
			);
			`); err != nil {
				t.Fatalf("seed: %v", err)
			}

			err := s.CreateClaim(ctx, types.NewClaim{
				HouseholdID: tt.householdID,
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

func TestGetClaimedHouseholdUnknownToken(t *testing.T) {
	s, _ := newTestStore(t)

	_, err := s.GetClaimedHousehold(context.Background(), "unknown-hash")
	if got, want := err, store.ErrClaimNotFound; got != want {
		t.Errorf("err=%v, want=%v", got, want)
	}
}

func TestAdminClaimOperations(t *testing.T) {
	s, d := newTestStore(t)
	ctx := context.Background()

	// Household 1 is claimed with token hash "old-hash". Household 2 is unclaimed.
	if _, err := d.Exec(ctx, `
	INSERT INTO household (id, kind, created_time, last_modified_time) VALUES
		(1, 'family', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z'),
		(2, 'family', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z');
	INSERT INTO household_member
		(household_id, first_name, label, created_time, last_modified_time)
	VALUES
		(1, 'Maria', 'Mom', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z'),
		(2, 'Dan', 'Dad', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z');
	INSERT INTO claim (
		household_id, giver_name, giver_email, giver_phone, access_token_hash,
		created_time
	) VALUES (
		1, 'Giver', 'giver@example.com', '555-0100', 'old-hash',
		'2026-11-05T14:30:00Z'
	);
	`); err != nil {
		t.Fatalf("seed: %v", err)
	}

	claim, err := s.GetClaim(ctx, 1)
	if err != nil {
		t.Fatalf("GetClaim: %v", err)
	}
	wantClaim := types.Claim{
		Giver: types.Giver{
			Name:  "Giver",
			Email: "giver@example.com",
			Phone: "555-0100",
		},
		CreatedTime: time.Date(2026, 11, 5, 14, 30, 0, 0, time.UTC),
	}
	if got, want := claim, wantClaim; !reflect.DeepEqual(got, want) {
		t.Errorf("claim=%+v, want=%+v", got, want)
	}

	if err := s.ReplaceClaimToken(ctx, 1, "new-hash"); err != nil {
		t.Fatalf("ReplaceClaimToken: %v", err)
	}
	if _, err := s.GetClaimedHousehold(ctx, "old-hash"); err != store.ErrClaimNotFound {
		t.Errorf("old token err=%v, want=%v", err, store.ErrClaimNotFound)
	}
	if _, err := s.GetClaimedHousehold(ctx, "new-hash"); err != nil {
		t.Errorf("new token err=%v, want=nil", err)
	}

	if err := s.DeleteClaim(ctx, 1); err != nil {
		t.Fatalf("DeleteClaim: %v", err)
	}
	if _, err := s.GetClaim(ctx, 1); err != store.ErrClaimNotFound {
		t.Errorf("GetClaim after release err=%v, want=%v", err, store.ErrClaimNotFound)
	}
	if _, err := s.GetUnclaimedHousehold(ctx, 1); err != nil {
		t.Errorf("GetUnclaimedHousehold after release err=%v, want=nil", err)
	}
}

func TestAdminClaimOperationsOnUnclaimedHousehold(t *testing.T) {
	s, d := newTestStore(t)
	ctx := context.Background()

	if _, err := d.Exec(ctx, `
	INSERT INTO household (id, kind, created_time, last_modified_time) VALUES
		(2, 'family', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z');
	`); err != nil {
		t.Fatalf("seed: %v", err)
	}

	if _, err := s.GetClaim(ctx, 2); err != store.ErrClaimNotFound {
		t.Errorf("GetClaim err=%v, want=%v", err, store.ErrClaimNotFound)
	}
	if err := s.DeleteClaim(ctx, 2); err != store.ErrClaimNotFound {
		t.Errorf("DeleteClaim err=%v, want=%v", err, store.ErrClaimNotFound)
	}
	if err := s.ReplaceClaimToken(ctx, 2, "new-hash"); err != store.ErrClaimNotFound {
		t.Errorf("ReplaceClaimToken err=%v, want=%v", err, store.ErrClaimNotFound)
	}
}

func TestGetHouseholdIncludesMemberlessHousehold(t *testing.T) {
	s, d := newTestStore(t)
	ctx := context.Background()

	if _, err := d.Exec(ctx, `
	INSERT INTO household (id, kind, created_time, last_modified_time) VALUES
		(3, 'family', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z');
	`); err != nil {
		t.Fatalf("seed: %v", err)
	}

	household, err := s.GetHousehold(ctx, 3)
	if err != nil {
		t.Fatalf("GetHousehold: %v", err)
	}
	if got, want := len(household.Members), 0; got != want {
		t.Errorf("len(Members)=%d, want=%d", got, want)
	}

	if _, err := s.GetHousehold(ctx, 99); err != store.ErrHouseholdNotFound {
		t.Errorf("missing household err=%v, want=%v", err, store.ErrHouseholdNotFound)
	}
}
