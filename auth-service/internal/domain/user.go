package domain

import "time"

type User struct {
	ID           string
	Fullname     string
	Email        string
	PasswordHash string
	RoleID       string
	IsActive     bool
	CreatedAt    time.Time
	UpdatedAt    time.Time
	DeletedAt    *time.Time
}
