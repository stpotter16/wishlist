package store

import (
	"context"
	"errors"

	"github.com/stpotter16/wishlist/internal/types"
)

var ErrAdminNotFound = errors.New("admin not found")
var ErrAdminExists = errors.New("admin already exists")
var ErrHouseholdNotFound = errors.New("household not found")
var ErrHouseholdAlreadyClaimed = errors.New("household is already claimed")
var ErrEmailAlreadyClaimed = errors.New("email has already claimed a household")
var ErrClaimNotFound = errors.New("claim not found")

type Store interface {
	// Admins
	GetAdminByUsername(ctx context.Context, username string) (types.Admin, error)
	CreateAdmin(ctx context.Context, username, passwordHash string) error

	// Households
	CreateHousehold(ctx context.Context, household types.Household) (int, error)
	UpdateHousehold(ctx context.Context, id int, household types.Household) error
	DeleteHousehold(ctx context.Context, id int) error
	ListHouseholds(ctx context.Context) ([]types.HouseholdSummary, error)
	ListUnclaimedHouseholds(ctx context.Context) ([]types.HouseholdSummary, error)
	GetUnclaimedHousehold(ctx context.Context, id int) (types.Household, error)
	GetHousehold(ctx context.Context, id int) (types.Household, error)

	// Claims
	CreateClaim(ctx context.Context, claim types.NewClaim) error
	GetClaimedHousehold(ctx context.Context, accessTokenHash string) (types.Household, error)
	GetClaim(ctx context.Context, householdID int) (types.Claim, error)
	DeleteClaim(ctx context.Context, householdID int) error
	ReplaceClaimToken(ctx context.Context, householdID int, accessTokenHash string) error
}
