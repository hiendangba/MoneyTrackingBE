package domain

import "time"

type Menu struct {
	ID        string
	Code      string
	Name      string
	Route     string
	Icon      *string
	ParentID  *string
	SortOrder int
	IsActive  bool
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt *time.Time
}
