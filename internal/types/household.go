package types

// HouseholdKind says who lives in a household.
type HouseholdKind string

const (
	// HouseholdKindFamily is a mom and her children.
	HouseholdKindFamily HouseholdKind = "family"
	// HouseholdKindSingle is one woman living on her own.
	HouseholdKindSingle HouseholdKind = "single"
)

// Label is how pages describe the kind of household.
func (k HouseholdKind) Label() string {
	if k == HouseholdKindSingle {
		return "Single woman"
	}
	return "Family"
}

// HouseholdSummary is the view of a household in a listing. It shows members
// only by their anonymous labels.
type HouseholdSummary struct {
	ID           int
	Kind         HouseholdKind
	MemberLabels []string
	IsClaimed    bool
}

// Household is the full view of a household and its members' wishlists. The
// first member is the head of the household: a mom or a single woman. When
// creating a household, the store ignores every ID field.
type Household struct {
	ID      int
	Kind    HouseholdKind
	Members []HouseholdMember
}

// HouseholdMember mirrors the paper intake sheet. Optional text fields are empty
// when the sheet left them blank. Only the head of the household has a last
// initial.
type HouseholdMember struct {
	ID                    int
	FirstName             string
	LastInitial           string
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

// Name is how pages name a member: "Maria G." for the head of the household,
// or just a first name for a child.
func (m HouseholdMember) Name() string {
	if m.LastInitial == "" {
		return m.FirstName
	}
	return m.FirstName + " " + m.LastInitial + "."
}

type WishlistItem struct {
	ID    int
	Name  string
	Notes string
}
