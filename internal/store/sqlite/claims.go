package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/stpotter16/wishlist/internal/store"
	"github.com/stpotter16/wishlist/internal/types"
)

// CreateClaim records a giver's claim on a family. It returns
// store.ErrFamilyNotFound if the family doesn't exist or has no members,
// store.ErrFamilyAlreadyClaimed if another giver claimed it first, and
// store.ErrEmailAlreadyClaimed if the email has claimed another family.
func (s Store) CreateClaim(ctx context.Context, claim types.NewClaim) error {
	now := formatTime(time.Now().UTC())

	// The checks and the insert share one transaction on the single write
	// connection, so concurrent claims can't both pass the checks. The UNIQUE
	// constraints on claim back this up.
	return s.db.WithTx(ctx, func(tx *sql.Tx) error {
		var hasMembers int
		if err := tx.QueryRowContext(ctx,
			`SELECT EXISTS (SELECT 1 FROM family_member WHERE family_id = ?)`,
			claim.FamilyID,
		).Scan(&hasMembers); err != nil {
			return err
		}
		if hasMembers == 0 {
			return store.ErrFamilyNotFound
		}

		var isFamilyClaimed int
		if err := tx.QueryRowContext(ctx,
			`SELECT EXISTS (SELECT 1 FROM claim WHERE family_id = ?)`,
			claim.FamilyID,
		).Scan(&isFamilyClaimed); err != nil {
			return err
		}
		if isFamilyClaimed != 0 {
			return store.ErrFamilyAlreadyClaimed
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
			family_id,
			giver_name,
			giver_email,
			giver_phone,
			access_token_hash,
			created_time
		) VALUES (?, ?, ?, ?, ?, ?)`,
			claim.FamilyID,
			claim.Giver.Name,
			claim.Giver.Email,
			claim.Giver.Phone,
			claim.AccessTokenHash,
			now,
		)
		return err
	})
}

// GetClaimedFamily returns the family claimed with the given access token
// hash. It returns store.ErrClaimNotFound if no claim has that hash.
func (s Store) GetClaimedFamily(ctx context.Context, accessTokenHash string) (types.Family, error) {
	var familyID int
	err := s.db.QueryRow(ctx,
		`SELECT family_id FROM claim WHERE access_token_hash = ?`,
		accessTokenHash,
	).Scan(&familyID)
	if errors.Is(err, sql.ErrNoRows) {
		return types.Family{}, store.ErrClaimNotFound
	}
	if err != nil {
		return types.Family{}, err
	}

	return s.loadFamily(ctx, familyID)
}
