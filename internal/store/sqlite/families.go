package sqlite

import (
	"context"

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
