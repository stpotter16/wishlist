package authentication

import (
	"context"
	"errors"
	"log"

	"github.com/stpotter16/wishlist/internal/auth"
	"github.com/stpotter16/wishlist/internal/store"
	"github.com/stpotter16/wishlist/internal/types"
)

var ErrInvalidCredentials = errors.New("invalid credentials")

type Authenticator struct {
	store store.Store
}

func New(store store.Store) Authenticator {
	return Authenticator{store: store}
}

func (a Authenticator) AuthenticateAdmin(ctx context.Context, req types.LoginRequest) (int, error) {
	admin, err := a.store.GetAdminByUsername(ctx, req.Username)
	if err != nil {
		if errors.Is(err, store.ErrAdminNotFound) {
			log.Printf("Authentication failed: admin not found")
			return 0, ErrInvalidCredentials
		}
		return 0, err
	}

	ok, err := auth.VerifyPassword(req.Password, admin.PasswordHash)
	if err != nil {
		log.Printf("Authentication failed: password verification error: %v", err)
		return 0, ErrInvalidCredentials
	}

	if !ok {
		log.Printf("Authentication failed: invalid password for admin %s", req.Username)
		return 0, ErrInvalidCredentials
	}

	log.Printf("Authentication succeeded for admin %s (id=%d)", req.Username, admin.ID)
	return admin.ID, nil
}
