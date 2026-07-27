package dto

import "time"

type AddMemberRequest struct {
	GroupID string
	UserID  string
	Role    string
}

type InviteMemberRequest struct {
	GroupID       string
	InvitedUserID string
	InvitedBy     string
	Role          string
	ExpiredAt     *time.Time
}
