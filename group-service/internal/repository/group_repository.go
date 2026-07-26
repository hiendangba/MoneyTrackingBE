package repository

import (
	"context"

	"group-service/internal/domain"
)

type GroupRepository interface {
	CreateGroup(ctx context.Context, group domain.Group) (*domain.Group, error)
	GetGroup(ctx context.Context, id string) (*domain.Group, error)
	ListGroups(ctx context.Context, userID string, includeInactive bool) ([]domain.Group, error)
	UpdateGroup(ctx context.Context, group domain.Group) (*domain.Group, error)
	SoftDeleteGroup(ctx context.Context, id string) error

	AddMember(ctx context.Context, member domain.GroupMember) (*domain.GroupMember, error)
	GetMember(ctx context.Context, groupID, userID string) (*domain.GroupMember, error)
	ListMembers(ctx context.Context, groupID string) ([]domain.GroupMember, error)
	RemoveMember(ctx context.Context, groupID, userID string) error

	CreateInvitation(ctx context.Context, invitation domain.GroupInvitation) (*domain.GroupInvitation, error)
	GetInvitation(ctx context.Context, id string) (*domain.GroupInvitation, error)
	ListInvitations(ctx context.Context, groupID string) ([]domain.GroupInvitation, error)
	UpdateInvitation(ctx context.Context, invitation domain.GroupInvitation) (*domain.GroupInvitation, error)
}

