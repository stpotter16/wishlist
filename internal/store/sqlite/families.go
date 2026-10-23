package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/stpotter16/wishlist/internal/store"
	"github.com/stpotter16/wishlist/internal/types"
)

// ListUnclaimedFamilies returns every family without a claim that has at least
// one member, ordered by family ID.
func (s Store) ListUnclaimedFamilies(ctx context.Context) ([]types.FamilySummary, error) {
	rows, err := s.db.Query(ctx, `
	SELECT
		family.id,
		family_member.label
	FROM
		family
	INNER JOIN family_member ON family_member.family_id = family.id
	WHERE
		NOT EXISTS (SELECT 1 FROM claim WHERE claim.family_id = family.id)
	ORDER BY
		family.id,
		family_member.id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	families := []types.FamilySummary{}
	for rows.Next() {
		var familyID int
		var memberLabel string
		if err := rows.Scan(&familyID, &memberLabel); err != nil {
			return nil, err
		}
		if len(families) == 0 || families[len(families)-1].ID != familyID {
			families = append(families, types.FamilySummary{ID: familyID})
		}
		last := &families[len(families)-1]
		last.MemberLabels = append(last.MemberLabels, memberLabel)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return families, nil
}

// GetUnclaimedFamily returns a family with its members and their wishlists.
// It returns store.ErrFamilyNotFound if the family doesn't exist, is claimed,
// or has no members, matching what ListUnclaimedFamilies shows.
func (s Store) GetUnclaimedFamily(ctx context.Context, id int) (types.Family, error) {
	rows, err := s.db.Query(ctx, `
	SELECT
		family_member.id,
		family_member.label,
		family_member.age,
		family_member.gender,
		family_member.shirt_size,
		family_member.pant_size,
		family_member.shoe_size,
		family_member.preferred_brands_colors,
		family_member.preferred_scents,
		family_member.prefers_unscented,
		family_member.preferred_gift_cards,
		family_member.additional_info
	FROM
		family_member
	WHERE
		family_member.family_id = ?
		AND NOT EXISTS (
			SELECT 1 FROM claim WHERE claim.family_id = family_member.family_id
		)
	ORDER BY
		family_member.id
	`, id)
	if err != nil {
		return types.Family{}, err
	}
	defer rows.Close()

	family := types.Family{ID: id}
	memberIndexByID := map[int]int{}
	for rows.Next() {
		var m types.FamilyMember
		var age sql.NullInt64
		var prefersUnscented int
		if err := rows.Scan(
			&m.ID,
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
			return types.Family{}, err
		}
		if age.Valid {
			a := int(age.Int64)
			m.Age = &a
		}
		m.PrefersUnscented = prefersUnscented != 0
		memberIndexByID[m.ID] = len(family.Members)
		family.Members = append(family.Members, m)
	}
	if err := rows.Err(); err != nil {
		return types.Family{}, err
	}
	if len(family.Members) == 0 {
		return types.Family{}, store.ErrFamilyNotFound
	}

	itemRows, err := s.db.Query(ctx, `
	SELECT
		wishlist_item.id,
		wishlist_item.family_member_id,
		wishlist_item.kind,
		wishlist_item.name,
		wishlist_item.notes
	FROM
		wishlist_item
	INNER JOIN
		family_member ON family_member.id = wishlist_item.family_member_id
	WHERE
		family_member.family_id = ?
	ORDER BY
		wishlist_item.id
	`, id)
	if err != nil {
		return types.Family{}, err
	}
	defer itemRows.Close()

	for itemRows.Next() {
		var item types.WishlistItem
		var memberID int
		var kind string
		if err := itemRows.Scan(&item.ID, &memberID, &kind, &item.Name, &item.Notes); err != nil {
			return types.Family{}, err
		}
		member := &family.Members[memberIndexByID[memberID]]
		switch kind {
		case "need":
			member.Needs = append(member.Needs, item)
		case "wish":
			member.Wishes = append(member.Wishes, item)
		default:
			return types.Family{}, fmt.Errorf("wishlist item %d has unknown kind %q", item.ID, kind)
		}
	}
	if err := itemRows.Err(); err != nil {
		return types.Family{}, err
	}

	return family, nil
}

// CreateFamily inserts a family with its members and their wishlist items in
// one transaction and returns the new family's ID.
func (s Store) CreateFamily(ctx context.Context, family types.Family) (int, error) {
	now := formatTime(time.Now().UTC())
	var familyID int64

	err := s.db.WithTx(ctx, func(tx *sql.Tx) error {
		result, err := tx.ExecContext(ctx,
			`INSERT INTO family (created_time, last_modified_time) VALUES (?, ?)`,
			now, now,
		)
		if err != nil {
			return err
		}
		familyID, err = result.LastInsertId()
		if err != nil {
			return err
		}

		for _, m := range family.Members {
			var age sql.NullInt64
			if m.Age != nil {
				age = sql.NullInt64{Int64: int64(*m.Age), Valid: true}
			}
			prefersUnscented := 0
			if m.PrefersUnscented {
				prefersUnscented = 1
			}

			result, err := tx.ExecContext(ctx, `
			INSERT INTO family_member (
				family_id,
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
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
				familyID,
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
	})
	if err != nil {
		return 0, err
	}

	return int(familyID), nil
}

func insertWishlistItems(ctx context.Context, tx *sql.Tx, memberID int64, kind string, items []types.WishlistItem, now string) error {
	for _, item := range items {
		if _, err := tx.ExecContext(ctx, `
		INSERT INTO wishlist_item (
			family_member_id,
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

// ListFamilies returns every family, including claimed families and families
// with no members, ordered by family ID.
func (s Store) ListFamilies(ctx context.Context) ([]types.FamilySummary, error) {
	rows, err := s.db.Query(ctx, `
	SELECT
		family.id,
		family_member.label,
		EXISTS (SELECT 1 FROM claim WHERE claim.family_id = family.id)
	FROM
		family
	LEFT JOIN family_member ON family_member.family_id = family.id
	ORDER BY
		family.id,
		family_member.id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	families := []types.FamilySummary{}
	for rows.Next() {
		var familyID int
		var memberLabel sql.NullString
		var isClaimed int
		if err := rows.Scan(&familyID, &memberLabel, &isClaimed); err != nil {
			return nil, err
		}
		if len(families) == 0 || families[len(families)-1].ID != familyID {
			families = append(families, types.FamilySummary{
				ID:        familyID,
				IsClaimed: isClaimed != 0,
			})
		}
		if memberLabel.Valid {
			last := &families[len(families)-1]
			last.MemberLabels = append(last.MemberLabels, memberLabel.String)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return families, nil
}
