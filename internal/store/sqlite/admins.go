package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/stpotter16/wishlist/internal/store"
	"github.com/stpotter16/wishlist/internal/types"
)

func (s Store) GetAdminByUsername(ctx context.Context, username string) (types.Admin, error) {
	row := s.db.QueryRow(ctx,
		`SELECT id, username, password_hash, created_time, last_modified_time FROM admin WHERE username = ?`,
		username,
	)

	var a types.Admin
	var createdTime, lastModifiedTime string

	err := row.Scan(&a.ID, &a.Username, &a.PasswordHash, &createdTime, &lastModifiedTime)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return types.Admin{}, store.ErrAdminNotFound
		}
		return types.Admin{}, err
	}

	a.CreatedTime, err = parseTime(createdTime)
	if err != nil {
		return types.Admin{}, err
	}

	a.LastModifiedTime, err = parseTime(lastModifiedTime)
	if err != nil {
		return types.Admin{}, err
	}

	return a, nil
}

func (s Store) CreateAdmin(ctx context.Context, username, passwordHash string) error {
	now := formatTime(time.Now().UTC())

	_, err := s.db.Exec(ctx,
		`INSERT INTO admin (username, password_hash, created_time, last_modified_time) VALUES (?, ?, ?, ?)`,
		username, passwordHash, now, now,
	)
	return err
}
