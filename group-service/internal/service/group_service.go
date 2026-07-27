package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/hiendangba/MoneyTrackingBE/Backend/group-service/internal/domain"
	dto "github.com/hiendangba/MoneyTrackingBE/Backend/group-service/internal/dto"
	apperrors "github.com/hiendangba/MoneyTrackingBE/Backend/group-service/internal/errors"
	"github.com/hiendangba/MoneyTrackingBE/Backend/group-service/internal/repository"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type GroupService struct {
	db   *pgxpool.Pool
	repo *repository.PostgresGroupRepository
}

func NewGroupService(db *pgxpool.Pool, repo *repository.PostgresGroupRepository) *GroupService {
	return &GroupService{db: db, repo: repo}
}

func (s *GroupService) CreateGroup(ctx context.Context, req dto.CreateGroupRequest) (*domain.Group, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, apperrors.Validation("name is required")
	}
	creator := strings.TrimSpace(req.CreatorUserID)
	if creator == "" {
		return nil, apperrors.Validation("creator_user_id is required")
	}

	now := time.Now()
	groupID, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("generate group id: %w", err)
	}

	var created *domain.Group
	if err := s.withTx(ctx, func(repo repository.GroupRepository) error {
		group, err := repo.CreateGroup(ctx, domain.Group{
			ID:          groupID.String(),
			Name:        name,
			Description: emptyStringToNil(req.Description),
			IsActive:    true,
			CreatedAt:   now,
			UpdatedAt:   now,
		})
		if err != nil {
			return err
		}
		created = group

		_, err = repo.AddMember(ctx, domain.GroupMember{
			GroupID:   group.ID,
			UserID:    creator,
			Role:      domain.MemberRoleOwner,
			JoinedAt:  now,
			CreatedAt: now,
			UpdatedAt: now,
		})
		return err
	}); err != nil {
		return nil, err
	}
	return created, nil
}

func (s *GroupService) GetGroup(ctx context.Context, id string) (*domain.Group, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, apperrors.Validation("id is required")
	}
	return s.repo.GetGroup(ctx, id)
}

func (s *GroupService) ListGroups(ctx context.Context, userID string, includeInactive bool) ([]domain.Group, error) {
	return s.repo.ListGroups(ctx, strings.TrimSpace(userID), includeInactive)
}

func (s *GroupService) UpdateGroup(ctx context.Context, id string, req dto.UpdateGroupRequest) (*domain.Group, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, apperrors.Validation("id is required")
	}

	group, err := s.repo.GetGroup(ctx, id)
	if err != nil {
		return nil, err
	}
	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			return nil, apperrors.Validation("name is required")
		}
		group.Name = name
	}
	if req.Description != nil {
		group.Description = emptyStringToNil(*req.Description)
	}
	if req.IsActive != nil {
		group.IsActive = *req.IsActive
	}
	group.UpdatedAt = time.Now()
	return s.repo.UpdateGroup(ctx, *group)
}

func (s *GroupService) DeleteGroup(ctx context.Context, id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return apperrors.Validation("id is required")
	}
	return s.repo.SoftDeleteGroup(ctx, id)
}

func (s *GroupService) ListMembers(ctx context.Context, groupID string) ([]domain.GroupMember, error) {
	groupID = strings.TrimSpace(groupID)
	if groupID == "" {
		return nil, apperrors.Validation("group_id is required")
	}
	return s.repo.ListMembers(ctx, groupID)
}

func (s *GroupService) GetMembership(ctx context.Context, groupID, userID string) (*domain.GroupMember, error) {
	groupID = strings.TrimSpace(groupID)
	userID = strings.TrimSpace(userID)
	if groupID == "" || userID == "" {
		return nil, apperrors.Validation("group_id and user_id are required")
	}
	return s.repo.GetMember(ctx, groupID, userID)
}

func (s *GroupService) AddMember(ctx context.Context, req dto.AddMemberRequest) (*domain.GroupMember, error) {
	groupID := strings.TrimSpace(req.GroupID)
	userID := strings.TrimSpace(req.UserID)
	if groupID == "" || userID == "" {
		return nil, apperrors.Validation("group_id and user_id are required")
	}

	group, err := s.repo.GetGroup(ctx, groupID)
	if err != nil {
		return nil, err
	}
	if !group.IsActive {
		return nil, apperrors.ErrGroupInactive
	}

	now := time.Now()
	memberRole := domain.MemberRole(strings.TrimSpace(req.Role))
	if memberRole == "" {
		memberRole = domain.MemberRoleMember
	}
	return s.repo.AddMember(ctx, domain.GroupMember{
		GroupID:   groupID,
		UserID:    userID,
		Role:      memberRole,
		JoinedAt:  now,
		CreatedAt: now,
		UpdatedAt: now,
	})
}

func (s *GroupService) RemoveMember(ctx context.Context, groupID, userID string) error {
	groupID = strings.TrimSpace(groupID)
	userID = strings.TrimSpace(userID)
	if groupID == "" || userID == "" {
		return apperrors.Validation("group_id and user_id are required")
	}
	return s.repo.RemoveMember(ctx, groupID, userID)
}

func (s *GroupService) InviteMember(ctx context.Context, req dto.InviteMemberRequest) (*domain.GroupInvitation, error) {
	groupID := strings.TrimSpace(req.GroupID)
	invitedUserID := strings.TrimSpace(req.InvitedUserID)
	invitedBy := strings.TrimSpace(req.InvitedBy)
	if groupID == "" || invitedUserID == "" || invitedBy == "" {
		return nil, apperrors.Validation("group_id, invited_user_id and invited_by are required")
	}

	group, err := s.repo.GetGroup(ctx, groupID)
	if err != nil {
		return nil, err
	}
	if !group.IsActive {
		return nil, apperrors.ErrGroupInactive
	}
	if _, err := s.repo.GetMember(ctx, groupID, invitedBy); err != nil {
		return nil, apperrors.ErrForbidden
	}

	now := time.Now()
	invitationID, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("generate invitation id: %w", err)
	}
	token, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("generate invitation token: %w", err)
	}
	role := domain.MemberRole(strings.TrimSpace(req.Role))
	if role == "" {
		role = domain.MemberRoleMember
	}
	status := domain.InvitationStatusPending
	invitation := domain.GroupInvitation{
		ID:            invitationID.String(),
		GroupID:       groupID,
		InvitedUserID: invitedUserID,
		InvitedBy:     invitedBy,
		Role:          role,
		Status:        status,
		Token:         token.String(),
		ExpiredAt:     req.ExpiredAt,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	return s.repo.CreateInvitation(ctx, invitation)
}

func (s *GroupService) ListInvitations(ctx context.Context, groupID string) ([]domain.GroupInvitation, error) {
	groupID = strings.TrimSpace(groupID)
	if groupID == "" {
		return nil, apperrors.Validation("group_id is required")
	}
	return s.repo.ListInvitations(ctx, groupID)
}

func (s *GroupService) RespondInvitation(ctx context.Context, invitationID string, accepted bool) (*domain.GroupInvitation, error) {
	invitationID = strings.TrimSpace(invitationID)
	if invitationID == "" {
		return nil, apperrors.Validation("invitation_id is required")
	}

	invitation, err := s.repo.GetInvitation(ctx, invitationID)
	if err != nil {
		return nil, err
	}
	if invitation.Status != domain.InvitationStatusPending {
		return nil, apperrors.ErrInvitationNotPending
	}
	if invitation.ExpiredAt != nil && time.Now().After(*invitation.ExpiredAt) {
		invitation.Status = domain.InvitationStatusExpired
		now := time.Now()
		invitation.RespondedAt = &now
		invitation.UpdatedAt = now
		if _, updateErr := s.repo.UpdateInvitation(ctx, *invitation); updateErr != nil {
			return nil, updateErr
		}
		return nil, apperrors.ErrInvitationExpired
	}

	now := time.Now()
	if accepted {
		if _, err := s.repo.GetMember(ctx, invitation.GroupID, invitation.InvitedUserID); err != nil {
			if !strings.Contains(err.Error(), apperrors.ErrGroupMemberNotFound.Error()) {
				return nil, err
			}
			if _, err := s.repo.AddMember(ctx, domain.GroupMember{
				GroupID:   invitation.GroupID,
				UserID:    invitation.InvitedUserID,
				Role:      invitation.Role,
				JoinedAt:  now,
				CreatedAt: now,
				UpdatedAt: now,
			}); err != nil && !strings.Contains(err.Error(), apperrors.ErrConflict.Error()) {
				return nil, err
			}
		}
		invitation.Status = domain.InvitationStatusAccepted
	} else {
		invitation.Status = domain.InvitationStatusRejected
	}
	invitation.RespondedAt = &now
	invitation.UpdatedAt = now
	return s.repo.UpdateInvitation(ctx, *invitation)
}

func (s *GroupService) withTx(ctx context.Context, fn func(repository.GroupRepository) error) error {
	tx, err := s.db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	txRepo := s.repo.WithDB(tx)
	if err := fn(txRepo); err != nil {
		if rollbackErr := tx.Rollback(ctx); rollbackErr != nil {
			return fmt.Errorf("%w: rollback: %v", err, rollbackErr)
		}
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
}

func emptyStringToNil(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return &value
}
