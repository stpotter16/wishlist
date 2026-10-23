package types

// FamilySummary is the public, anonymous view of a family in a listing.
type FamilySummary struct {
	ID           int
	MemberLabels []string
}

// Family is the full, anonymous view of a family and its members' wishlists.
type Family struct {
	ID      int
	Members []FamilyMember
}

// FamilyMember mirrors the paper intake sheet. Optional text fields are empty
// when the sheet left them blank.
type FamilyMember struct {
	ID                    int
	Label                 string
	Age                   *int
	Gender                string
	ShirtSize             string
	PantSize              string
	ShoeSize              string
	PreferredBrandsColors string
	PreferredScents       string
	PrefersUnscented      bool
	PreferredGiftCards    string
	AdditionalInfo        string
	Needs                 []WishlistItem
	Wishes                []WishlistItem
}

type WishlistItem struct {
	ID    int
	Name  string
	Notes string
}
