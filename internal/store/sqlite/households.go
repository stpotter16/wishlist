package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/stpotter16/wishlist/internal/store"
	"github.com/stpotter16/wishlist/internal/types"
)

// ListUnclaimedHouseholds returns every household without a claim that has at least
// one member, ordered by household ID.
func (s Store) ListUnclaimedHouseholds(ctx context.Context) ([]types.HouseholdSummary, error) {
	rows, err := s.db.Query(ctx, `
	SELECT
		household.id,
		household.kind,
		household_member.label
	FROM
		household
	INNER JOIN household_member ON household_member.household_id = household.id
	WHERE
		NOT EXISTS (SELECT 1 FROM claim WHERE claim.household_id = household.id)
	ORDER BY
		household.id,
		household_member.id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	households := []types.HouseholdSummary{}
	for rows.Next() {
		var householdID int
		var kind string
		var memberLabel string
		if err := rows.Scan(&householdID, &kind, &memberLabel); err != nil {
			return nil, err
		}
		if len(households) == 0 || households[len(households)-1].ID != householdID {
			households = append(households, types.HouseholdSummary{
				ID:   householdID,
				Kind: types.HouseholdKind(kind),
			})
		}
		last := &households[len(households)-1]
		last.MemberLabels = append(last.MemberLabels, memberLabel)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return households, nil
}

// GetUnclaimedHousehold returns a household with its members and their wishlists.
// It returns store.ErrHouseholdNotFound if the household doesn't exist, is claimed,
// or has no members, matching what ListUnclaimedHouseholds shows.
func (s Store) GetUnclaimedHousehold(ctx context.Context, id int) (types.Household, error) {
	var isClaimed int
	if err := s.db.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM claim WHERE household_id = ?)`,
		id,
	).Scan(&isClaimed); err != nil {
		return types.Household{}, err
	}
	if isClaimed != 0 {
		return types.Household{}, store.ErrHouseholdNotFound
	}

	household, err := s.loadHousehold(ctx, id)
	if err != nil {
		return types.Household{}, err
	}
	if len(household.Members) == 0 {
		return types.Household{}, store.ErrHouseholdNotFound
	}
	return household, nil
}

// GetHousehold returns any household with its members and their wishlists, for
// admins. It returns store.ErrHouseholdNotFound if the household doesn't exist.
func (s Store) GetHousehold(ctx context.Context, id int) (types.Household, error) {
	return s.loadHousehold(ctx, id)
}

// loadHousehold returns a household with its members and their wishlists, whether
// or not it is claimed or has members. It returns store.ErrHouseholdNotFound if
// the household doesn't exist.
func (s Store) loadHousehold(ctx context.Context, id int) (types.Household, error) {
	var kind string
	err := s.db.QueryRow(ctx,
		`SELECT kind FROM household WHERE id = ?`,
		id,
	).Scan(&kind)
	if errors.Is(err, sql.ErrNoRows) {
		return types.Household{}, store.ErrHouseholdNotFound
	}
	if err != nil {
		return types.Household{}, err
	}

	rows, err := s.db.Query(ctx, `
	SELECT
		household_member.id,
		household_member.first_name,
		household_member.last_initial,
		household_member.label,
		household_member.age,
		household_member.gender,
		household_member.shirt_size,
		household_member.pant_size,
		household_member.shoe_size,
		household_member.preferred_brands_colors,
		household_member.preferred_scents,
		household_member.prefers_unscented,
		household_member.preferred_gift_cards,
		household_member.additional_info
	FROM
		household_member
	WHERE
		household_member.household_id = ?
	ORDER BY
		household_member.id
	`, id)
	if err != nil {
		return types.Household{}, err
	}
	defer rows.Close()

	household := types.Household{ID: id, Kind: types.HouseholdKind(kind)}
	memberIndexByID := map[int]int{}
	for rows.Next() {
		var m types.HouseholdMember
		var age sql.NullInt64
		var prefersUnscented int
		if err := rows.Scan(
			&m.ID,
			&m.FirstName,
			&m.LastInitial,
			&m.Label,
			&age,
			&m.Gender,
			&m.ShirtSize,
			&m.PantSize,
			&m.ShoeSize,
			&m.PreferredBrandsColors,
			&m.PreferredScents,
			&prefersUnscented,
			&m.PreferredGiftCards,
			&m.AdditionalInfo,
		); err != nil {
			return types.Household{}, err
		}
		if age.Valid {
			a := int(age.Int64)
			m.Age = &a
		}
		m.PrefersUnscented = prefersUnscented != 0
		memberIndexByID[m.ID] = len(household.Members)
		household.Members = append(household.Members, m)
	}
	if err := rows.Err(); err != nil {
		return types.Household{}, err
	}
	itemRows, err := s.db.Query(ctx, `
	SELECT
		wishlist_item.id,
		wishlist_item.household_member_id,
		wishlist_item.kind,
		wishlist_item.name,
		wishlist_item.notes
	FROM
		wishlist_item
	INNER JOIN
		household_member ON household_member.id = wishlist_item.household_member_id
	WHERE
		household_member.household_id = ?
	ORDER BY
		wishlist_item.id
	`, id)
	if err != nil {
		return types.Household{}, err
	}
	defer itemRows.Close()

	for itemRows.Next() {
		var item types.WishlistItem
		var memberID int
		var kind string
		if err := itemRows.Scan(&item.ID, &memberID, &kind, &item.Name, &item.Notes); err != nil {
			return types.Household{}, err
		}
		member := &household.Members[memberIndexByID[memberID]]
		switch kind {
		case "need":
			member.Needs = append(member.Needs, item)
		case "wish":
			member.Wishes = append(member.Wishes, item)
		default:
			return types.Household{}, fmt.Errorf("wishlist item %d has unknown kind %q", item.ID, kind)
		}
	}
	if err := itemRows.Err(); err != nil {
		return types.Household{}, err
	}

	return household, nil
}

// CreateHousehold inserts a household with its members and their wishlist items in
// one transaction and returns the new household's ID.
func (s Store) CreateHousehold(ctx context.Context, household types.Household) (int, error) {
	now := formatTime(time.Now().UTC())
	var householdID int64

	err := s.db.WithTx(ctx, func(tx *sql.Tx) error {
		result, err := tx.ExecContext(ctx,
			`INSERT INTO household (kind, created_time, last_modified_time) VALUES (?, ?, ?)`,
			household.Kind, now, now,
		)
		if err != nil {
			return err
		}
		householdID, err = result.LastInsertId()
		if err != nil {
			return err
		}

		return insertMembers(ctx, tx, householdID, household.Members, now)
	})
	if err != nil {
		return 0, err
	}

	return int(householdID), nil
}

// UpdateHousehold replaces a household's kind and members and their wishlist items in one
// transaction. Members and items get new IDs. A claim on the household is kept.
// It returns store.ErrHouseholdNotFound if the household doesn't exist.
func (s Store) UpdateHousehold(ctx context.Context, id int, household types.Household) error {
	now := formatTime(time.Now().UTC())

	return s.db.WithTx(ctx, func(tx *sql.Tx) error {
		result, err := tx.ExecContext(ctx,
			`UPDATE household SET kind = ?, last_modified_time = ? WHERE id = ?`,
			household.Kind, now, id,
		)
		if err != nil {
			return err
		}
		if err := requireOneRow(result, store.ErrHouseholdNotFound); err != nil {
			return err
		}

		// Deleting members cascades to their wishlist items.
		if _, err := tx.ExecContext(ctx,
			`DELETE FROM household_member WHERE household_id = ?`,
			id,
		); err != nil {
			return err
		}

		return insertMembers(ctx, tx, int64(id), household.Members, now)
	})
}

// DeleteHousehold deletes an unclaimed household with its members and their
// wishlist items. It returns store.ErrHouseholdNotFound if the household doesn't
// exist and store.ErrHouseholdAlreadyClaimed if a giver has claimed it.
func (s Store) DeleteHousehold(ctx context.Context, id int) error {
	return s.db.WithTx(ctx, func(tx *sql.Tx) error {
		var isClaimed int
		if err := tx.QueryRowContext(ctx,
			`SELECT EXISTS (SELECT 1 FROM claim WHERE household_id = ?)`,
			id,
		).Scan(&isClaimed); err != nil {
			return err
		}
		if isClaimed != 0 {
			return store.ErrHouseholdAlreadyClaimed
		}

		// Deleting the household cascades to its members and their wishlist items.
		result, err := tx.ExecContext(ctx, `DELETE FROM household WHERE id = ?`, id)
		if err != nil {
			return err
		}
		return requireOneRow(result, store.ErrHouseholdNotFound)
	})
}

// insertMembers inserts members and their wishlist items into a household.
func insertMembers(ctx context.Context, tx *sql.Tx, householdID int64, members []types.HouseholdMember, now string) error {
	for _, m := range members {
		var age sql.NullInt64
		if m.Age != nil {
			age = sql.NullInt64{Int64: int64(*m.Age), Valid: true}
		}
		prefersUnscented := 0
		if m.PrefersUnscented {
			prefersUnscented = 1
		}

		result, err := tx.ExecContext(ctx, `
		INSERT INTO household_member (
			household_id,
			first_name,
			last_initial,
			label,
			age,
			gender,
			shirt_size,
			pant_size,
			shoe_size,
			preferred_brands_colors,
			preferred_scents,
			prefers_unscented,
			preferred_gift_cards,
			additional_info,
			created_time,
			last_modified_time
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			householdID,
			m.FirstName,
			m.LastInitial,
			m.Label,
			age,
			m.Gender,
			m.ShirtSize,
			m.PantSize,
			m.ShoeSize,
			m.PreferredBrandsColors,
			m.PreferredScents,
			prefersUnscented,
			m.PreferredGiftCards,
			m.AdditionalInfo,
			now,
			now,
		)
		if err != nil {
			return err
		}
		memberID, err := result.LastInsertId()
		if err != nil {
			return err
		}

		if err := insertWishlistItems(ctx, tx, memberID, "need", m.Needs, now); err != nil {
			return err
		}
		if err := insertWishlistItems(ctx, tx, memberID, "wish", m.Wishes, now); err != nil {
			return err
		}
	}
	return nil
}

func insertWishlistItems(ctx context.Context, tx *sql.Tx, memberID int64, kind string, items []types.WishlistItem, now string) error {
	for _, item := range items {
		if _, err := tx.ExecContext(ctx, `
		INSERT INTO wishlist_item (
			household_member_id,
			kind,
			name,
			notes,
			created_time,
			last_modified_time
		) VALUES (?, ?, ?, ?, ?, ?)`,
			memberID, kind, item.Name, item.Notes, now, now,
		); err != nil {
			return err
		}
	}
	return nil
}

// ListHouseholds returns every household, including claimed households and households
// with no members, ordered by household ID. Claimed households include the giver's
// name and email.
func (s Store) ListHouseholds(ctx context.Context) ([]types.HouseholdSummary, error) {
	rows, err := s.db.Query(ctx, `
	SELECT
		household.id,
		household.kind,
		household_member.label,
		claim.giver_name,
		claim.giver_email
	FROM
		household
	LEFT JOIN household_member ON household_member.household_id = household.id
	LEFT JOIN claim ON claim.household_id = household.id
	ORDER BY
		household.id,
		household_member.id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	households := []types.HouseholdSummary{}
	for rows.Next() {
		var householdID int
		var kind string
		var memberLabel sql.NullString
		var giverName sql.NullString
		var giverEmail sql.NullString
		if err := rows.Scan(&householdID, &kind, &memberLabel, &giverName, &giverEmail); err != nil {
			return nil, err
		}
		if len(households) == 0 || households[len(households)-1].ID != householdID {
			households = append(households, types.HouseholdSummary{
				ID:         householdID,
				Kind:       types.HouseholdKind(kind),
				IsClaimed:  giverName.Valid,
				GiverName:  giverName.String,
				GiverEmail: giverEmail.String,
			})
		}
		if memberLabel.Valid {
			last := &households[len(households)-1]
			last.MemberLabels = append(last.MemberLabels, memberLabel.String)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return households, nil
}
