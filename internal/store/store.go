package store

import (
	"context"
	"errors"

	"github.com/stpotter16/wishlist/internal/types"
)

var ErrAdminNotFound = errors.New("admin not found")
var ErrAdminExists = errors.New("admin already exists")

type Store interface {
	// Admins
	GetAdminByUsername(ctx context.Context, username string) (types.Admin, error)
	CreateAdmin(ctx context.Context, username, passwordHash string) error
}
