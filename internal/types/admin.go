package types

import "time"

type Admin struct {
	ID               int
	Username         string
	PasswordHash     string
	CreatedTime      time.Time
	LastModifiedTime time.Time
}
