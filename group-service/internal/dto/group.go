package dto

import "time"

type CreateGroupRequest struct {
	Name          string `json:"name"`
	Description   string `json:"description"`
	CreatorUserID string `json:"creator_user_id"`
}

type UpdateGroupRequest struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
	IsActive    *bool   `json:"is_active"`
}

type GroupResponse struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description *string   `json:"description,omitempty"`
	IsActive    bool      `json:"is_active"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type GroupMemberResponse struct {
	GroupID   string    `json:"group_id"`
	UserID    string    `json:"user_id"`
	Role      string    `json:"role"`
	JoinedAt  time.Time `json:"joined_at"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type GroupInvitationResponse struct {
	ID            string     `json:"id"`
	GroupID       string     `json:"group_id"`
	InvitedUserID string     `json:"invited_user_id"`
	InvitedBy     string     `json:"invited_by"`
	Role          string     `json:"role"`
	Status        string     `json:"status"`
	Token         string     `json:"token"`
	ExpiredAt     *time.Time `json:"expired_at,omitempty"`
	RespondedAt   *time.Time `json:"responded_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}
