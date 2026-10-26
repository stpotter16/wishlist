package types

import "time"

// Giver is the contact information a giver leaves when claiming a household.
// Only admins can read it.
type Giver struct {
	Name  string
	Email string
	Phone string
}

// NewClaim is a giver's claim on a household. The store keeps only the hash of
// the giver's access token.
type NewClaim struct {
	HouseholdID     int
	Giver           Giver
	AccessTokenHash string
}

// Claim is a giver's claim on a household as admins see it.
type Claim struct {
	Giver       Giver
	CreatedTime time.Time
}
