package domain

import "time"

type User struct {
	ID             string
	Fullname       string
	Email          string
	PasswordHash   string
	RoleID         string
	RoleCode       string
	RoleActive     bool
	SessionVersion int64
	IsActive       bool
	CreatedAt      time.Time
	UpdatedAt      time.Time
	DeletedAt      *time.Time
}
