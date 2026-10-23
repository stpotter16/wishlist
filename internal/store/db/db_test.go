package db_test

import (
	"context"
	"testing"

	"github.com/stpotter16/wishlist/internal/store/db"
)

func TestPragmasApplyToEveryReadConnection(t *testing.T) {
	d, err := db.New(t.TempDir())
	if err != nil {
		t.Fatalf("db.New: %v", err)
	}
	ctx := context.Background()

	// Hold the first connection open so the next query has to open a second
	// one.
	held, err := d.Query(ctx, `PRAGMA foreign_keys`)
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	defer held.Close()

	var foreignKeys int
	if err := d.QueryRow(ctx, `PRAGMA foreign_keys`).Scan(&foreignKeys); err != nil {
		t.Fatalf("QueryRow: %v", err)
	}

	if got, want := foreignKeys, 1; got != want {
		t.Errorf("foreign_keys=%d, want=%d", got, want)
	}
}
