package domain

import "time"

type Group struct {
	ID          string
	Name        string
	Description *string
	IsActive    bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
	DeletedAt   *time.Time
}

type MemberRole string

const (
	MemberRoleOwner  MemberRole = "owner"
	MemberRoleMember MemberRole = "member"
)

type GroupMember struct {
	GroupID   string
	UserID    string
	Role      MemberRole
	JoinedAt  time.Time
	CreatedAt time.Time
	UpdatedAt time.Time
}

type InvitationStatus string

const (
	InvitationStatusPending  InvitationStatus = "pending"
	InvitationStatusAccepted InvitationStatus = "accepted"
	InvitationStatusRejected InvitationStatus = "rejected"
	InvitationStatusExpired   InvitationStatus = "expired"
)

type GroupInvitation struct {
	ID            string
	GroupID       string
	InvitedUserID string
	InvitedBy     string
	Role          MemberRole
	Status        InvitationStatus
	Token         string
	ExpiredAt     *time.Time
	RespondedAt   *time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time
	DeletedAt     *time.Time
}

