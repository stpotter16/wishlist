package types

// FamilySummary is the public, anonymous view of a family in a listing.
type FamilySummary struct {
	ID           int
	MemberLabels []string
}
