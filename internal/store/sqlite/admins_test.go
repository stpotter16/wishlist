package sqlite_test

import (
	"context"
	"testing"

	"github.com/stpotter16/wishlist/internal/store"
)

func TestCreateAdmin(t *testing.T) {
	for _, tt := range []struct {
		explanation      string
		existingUsername string
		newUsername      string
		errExpected      error
	}{
		{
			"new username is created",
			"alice",
			"bob",
			nil,
		},
		{
			"duplicate username is rejected",
			"alice",
			"alice",
			store.ErrAdminExists,
		},
	} {
		t.Run(tt.explanation, func(t *testing.T) {
			s, _ := newTestStore(t)
			ctx := context.Background()

			if err := s.CreateAdmin(ctx, tt.existingUsername, "dummy-hash"); err != nil {
				t.Fatalf("CreateAdmin(%q): %v", tt.existingUsername, err)
			}

			err := s.CreateAdmin(ctx, tt.newUsername, "dummy-hash")
			if got, want := err, tt.errExpected; got != want {
				t.Fatalf("err=%v, want=%v", got, want)
			}
		})
	}
}

func TestGetAdminByUsername(t *testing.T) {
	s, _ := newTestStore(t)
	ctx := context.Background()

	if err := s.CreateAdmin(ctx, "alice", "alice-hash"); err != nil {
		t.Fatalf("CreateAdmin: %v", err)
	}

	admin, err := s.GetAdminByUsername(ctx, "alice")
	if err != nil {
		t.Fatalf("GetAdminByUsername: %v", err)
	}
	if got, want := admin.PasswordHash, "alice-hash"; got != want {
		t.Errorf("PasswordHash=%q, want=%q", got, want)
	}

	if _, err := s.GetAdminByUsername(ctx, "nobody"); err != store.ErrAdminNotFound {
		t.Errorf("err=%v, want=%v", err, store.ErrAdminNotFound)
	}
}
