package store

import (
	"context"
	"errors"

	"github.com/stpotter16/wishlist/internal/types"
)

var ErrAdminNotFound = errors.New("admin not found")
var ErrAdminExists = errors.New("admin already exists")
var ErrFamilyNotFound = errors.New("family not found")
var ErrFamilyAlreadyClaimed = errors.New("family is already claimed")
var ErrEmailAlreadyClaimed = errors.New("email has already claimed a family")
var ErrClaimNotFound = errors.New("claim not found")

type Store interface {
	// Admins
	GetAdminByUsername(ctx context.Context, username string) (types.Admin, error)
	CreateAdmin(ctx context.Context, username, passwordHash string) error

	// Families
	CreateFamily(ctx context.Context, family types.Family) (int, error)
	ListFamilies(ctx context.Context) ([]types.FamilySummary, error)
	ListUnclaimedFamilies(ctx context.Context) ([]types.FamilySummary, error)
	GetUnclaimedFamily(ctx context.Context, id int) (types.Family, error)
	GetFamily(ctx context.Context, id int) (types.Family, error)

	// Claims
	CreateClaim(ctx context.Context, claim types.NewClaim) error
	GetClaimedFamily(ctx context.Context, accessTokenHash string) (types.Family, error)
	GetClaim(ctx context.Context, familyID int) (types.Claim, error)
	DeleteClaim(ctx context.Context, familyID int) error
	ReplaceClaimToken(ctx context.Context, familyID int, accessTokenHash string) error
}
