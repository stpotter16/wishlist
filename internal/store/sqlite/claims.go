package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/stpotter16/wishlist/internal/store"
	"github.com/stpotter16/wishlist/internal/types"
)

// CreateClaim records a giver's claim on a household. It returns
// store.ErrHouseholdNotFound if the household doesn't exist or has no members,
// store.ErrHouseholdAlreadyClaimed if another giver claimed it first, and
// store.ErrEmailAlreadyClaimed if the email has claimed another household.
func (s Store) CreateClaim(ctx context.Context, claim types.NewClaim) error {
	now := formatTime(time.Now().UTC())

	// The checks and the insert share one transaction on the single write
	// connection, so concurrent claims can't both pass the checks. The UNIQUE
	// constraints on claim back this up.
	return s.db.WithTx(ctx, func(tx *sql.Tx) error {
		var hasMembers int
		if err := tx.QueryRowContext(ctx,
			`SELECT EXISTS (SELECT 1 FROM household_member WHERE household_id = ?)`,
			claim.HouseholdID,
		).Scan(&hasMembers); err != nil {
			return err
		}
		if hasMembers == 0 {
			return store.ErrHouseholdNotFound
		}

		var isHouseholdClaimed int
		if err := tx.QueryRowContext(ctx,
			`SELECT EXISTS (SELECT 1 FROM claim WHERE household_id = ?)`,
			claim.HouseholdID,
		).Scan(&isHouseholdClaimed); err != nil {
			return err
		}
		if isHouseholdClaimed != 0 {
			return store.ErrHouseholdAlreadyClaimed
		}

		// giver_email uses COLLATE nocase, so this match ignores case.
		var isEmailUsed int
		if err := tx.QueryRowContext(ctx,
			`SELECT EXISTS (SELECT 1 FROM claim WHERE giver_email = ?)`,
			claim.Giver.Email,
		).Scan(&isEmailUsed); err != nil {
			return err
		}
		if isEmailUsed != 0 {
			return store.ErrEmailAlreadyClaimed
		}

		_, err := tx.ExecContext(ctx, `
		INSERT INTO claim (
			household_id,
			giver_name,
			giver_email,
			giver_phone,
			access_token_hash,
			created_time
		) VALUES (?, ?, ?, ?, ?, ?)`,
			claim.HouseholdID,
			claim.Giver.Name,
			claim.Giver.Email,
			claim.Giver.Phone,
			claim.AccessTokenHash,
			now,
		)
		return err
	})
}

// GetClaimedHousehold returns the household claimed with the given access token
// hash. It returns store.ErrClaimNotFound if no claim has that hash.
func (s Store) GetClaimedHousehold(ctx context.Context, accessTokenHash string) (types.Household, error) {
	var householdID int
	err := s.db.QueryRow(ctx,
		`SELECT household_id FROM claim WHERE access_token_hash = ?`,
		accessTokenHash,
	).Scan(&householdID)
	if errors.Is(err, sql.ErrNoRows) {
		return types.Household{}, store.ErrClaimNotFound
	}
	if err != nil {
		return types.Household{}, err
	}

	return s.loadHousehold(ctx, householdID)
}

// GetClaim returns the claim on a household. It returns store.ErrClaimNotFound if
// the household is unclaimed.
func (s Store) GetClaim(ctx context.Context, householdID int) (types.Claim, error) {
	var c types.Claim
	var createdTime string
	err := s.db.QueryRow(ctx, `
	SELECT
		giver_name,
		giver_email,
		giver_phone,
		created_time
	FROM
		claim
	WHERE
		household_id = ?
	`, householdID).Scan(&c.Giver.Name, &c.Giver.Email, &c.Giver.Phone, &createdTime)
	if errors.Is(err, sql.ErrNoRows) {
		return types.Claim{}, store.ErrClaimNotFound
	}
	if err != nil {
		return types.Claim{}, err
	}

	c.CreatedTime, err = parseTime(createdTime)
	if err != nil {
		return types.Claim{}, err
	}

	return c, nil
}

// DeleteClaim releases a household's claim, which returns the household to the
// public listing and invalidates the giver's private link. It returns
// store.ErrClaimNotFound if the household is unclaimed.
func (s Store) DeleteClaim(ctx context.Context, householdID int) error {
	result, err := s.db.Exec(ctx, `DELETE FROM claim WHERE household_id = ?`, householdID)
	if err != nil {
		return err
	}
	return requireOneRow(result, store.ErrClaimNotFound)
}

// ReplaceClaimToken swaps a claim's access token hash, which invalidates the
// giver's old private link. It returns store.ErrClaimNotFound if the household is
// unclaimed.
func (s Store) ReplaceClaimToken(ctx context.Context, householdID int, accessTokenHash string) error {
	result, err := s.db.Exec(ctx,
		`UPDATE claim SET access_token_hash = ? WHERE household_id = ?`,
		accessTokenHash, householdID,
	)
	if err != nil {
		return err
	}
	return requireOneRow(result, store.ErrClaimNotFound)
}

func requireOneRow(result sql.Result, errNoRows error) error {
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return errNoRows
	}
	return nil
}
