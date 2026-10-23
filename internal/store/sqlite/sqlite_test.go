package sqlite_test

import (
	"testing"

	"github.com/stpotter16/wishlist/internal/store/db"
	"github.com/stpotter16/wishlist/internal/store/sqlite"
)

// newTestStore returns a migrated store and its underlying database, so tests
// can seed rows that the store has no methods to create yet.
func newTestStore(t *testing.T) (sqlite.Store, db.DB) {
	t.Helper()

	d, err := db.New(t.TempDir())
	if err != nil {
		t.Fatalf("db.New: %v", err)
	}

	s, err := sqlite.New(d)
	if err != nil {
		t.Fatalf("sqlite.New: %v", err)
	}

	return s, d
}
