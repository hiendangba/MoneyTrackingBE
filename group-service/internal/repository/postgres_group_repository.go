package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/hiendangba/MoneyTrackingBE/Backend/group-service/internal/domain"
	apperrors "github.com/hiendangba/MoneyTrackingBE/Backend/group-service/internal/errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type dbExecutor interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
}

type PostgresGroupRepository struct {
	db dbExecutor
}

func NewPostgresGroupRepository(db dbExecutor) *PostgresGroupRepository {
	return &PostgresGroupRepository{db: db}
}

func (r *PostgresGroupRepository) WithDB(db dbExecutor) *PostgresGroupRepository {
	return &PostgresGroupRepository{db: db}
}

func (r *PostgresGroupRepository) CreateGroup(ctx context.Context, group domain.Group) (*domain.Group, error) {
	query := `
		INSERT INTO groups (id, name, description, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, name, description, is_active, created_at, updated_at, deleted_at
	`
	created, err := scanGroup(r.db.QueryRow(ctx, query, group.ID, group.Name, group.Description, group.IsActive, group.CreatedAt, group.UpdatedAt))
	if err != nil {
		if isConflict(err) {
			return nil, apperrors.ErrConflict
		}
		return nil, fmt.Errorf("create group: %w", err)
	}
	return created, nil
}

func (r *PostgresGroupRepository) GetGroup(ctx context.Context, id string) (*domain.Group, error) {
	query := `
		SELECT id, name, description, is_active, created_at, updated_at, deleted_at
		FROM groups
		WHERE id = $1 AND deleted_at IS NULL
	`
	group, err := scanGroup(r.db.QueryRow(ctx, query, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.ErrGroupNotFound
		}
		return nil, fmt.Errorf("get group: %w", err)
	}
	return group, nil
}

func (r *PostgresGroupRepository) ListGroups(ctx context.Context, userID string, includeInactive bool) ([]domain.Group, error) {
	var (
		query string
		args  []any
	)
	if strings.TrimSpace(userID) != "" {
		query = `
			SELECT g.id, g.name, g.description, g.is_active, g.created_at, g.updated_at, g.deleted_at
			FROM groups g
			JOIN group_members gm ON gm.group_id = g.id
			WHERE gm.user_id = $1 AND g.deleted_at IS NULL
		`
		args = append(args, userID)
		if !includeInactive {
			query += " AND g.is_active = TRUE"
		}
		query += " ORDER BY g.created_at ASC"
	} else {
		query = `
			SELECT id, name, description, is_active, created_at, updated_at, deleted_at
			FROM groups
			WHERE deleted_at IS NULL
		`
		if !includeInactive {
			query += " AND is_active = TRUE"
		}
		query += " ORDER BY created_at ASC"
	}
	return r.listGroupsByQuery(ctx, query, args...)
}

func (r *PostgresGroupRepository) UpdateGroup(ctx context.Context, group domain.Group) (*domain.Group, error) {
	query := `
		UPDATE groups
		SET name = $1,
			description = $2,
			is_active = $3,
			updated_at = $4
		WHERE id = $5 AND deleted_at IS NULL
		RETURNING id, name, description, is_active, created_at, updated_at, deleted_at
	`
	updated, err := scanGroup(r.db.QueryRow(ctx, query, group.Name, group.Description, group.IsActive, group.UpdatedAt, group.ID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.ErrGroupNotFound
		}
		if isConflict(err) {
			return nil, apperrors.ErrConflict
		}
		return nil, fmt.Errorf("update group: %w", err)
	}
	return updated, nil
}

func (r *PostgresGroupRepository) SoftDeleteGroup(ctx context.Context, id string) error {
	query := `UPDATE groups SET deleted_at = NOW(), updated_at = NOW() WHERE id = $1 AND deleted_at IS NULL`
	tag, err := r.db.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("soft delete group: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return apperrors.ErrGroupNotFound
	}
	return nil
}

func (r *PostgresGroupRepository) AddMember(ctx context.Context, member domain.GroupMember) (*domain.GroupMember, error) {
	query := `
		INSERT INTO group_members (group_id, user_id, role, joined_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING group_id, user_id, role, joined_at, created_at, updated_at
	`
	created, err := scanMember(r.db.QueryRow(ctx, query, member.GroupID, member.UserID, member.Role, member.JoinedAt, member.CreatedAt, member.UpdatedAt))
	if err != nil {
		if isConflict(err) {
			return nil, apperrors.ErrConflict
		}
		return nil, fmt.Errorf("add member: %w", err)
	}
	return created, nil
}

func (r *PostgresGroupRepository) GetMember(ctx context.Context, groupID, userID string) (*domain.GroupMember, error) {
	query := `
		SELECT group_id, user_id, role, joined_at, created_at, updated_at
		FROM group_members
		WHERE group_id = $1 AND user_id = $2
	`
	member, err := scanMember(r.db.QueryRow(ctx, query, groupID, userID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.ErrGroupMemberNotFound
		}
		return nil, fmt.Errorf("get member: %w", err)
	}
	return member, nil
}

func (r *PostgresGroupRepository) ListMembers(ctx context.Context, groupID string) ([]domain.GroupMember, error) {
	query := `
		SELECT group_id, user_id, role, joined_at, created_at, updated_at
		FROM group_members
		WHERE group_id = $1
		ORDER BY joined_at ASC
	`
	rows, err := r.db.Query(ctx, query, groupID)
	if err != nil {
		return nil, fmt.Errorf("list members: %w", err)
	}
	defer rows.Close()

	members := make([]domain.GroupMember, 0)
	for rows.Next() {
		member, scanErr := scanMember(rows)
		if scanErr != nil {
			return nil, fmt.Errorf("scan member: %w", scanErr)
		}
		members = append(members, *member)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate members: %w", err)
	}
	return members, nil
}

func (r *PostgresGroupRepository) RemoveMember(ctx context.Context, groupID, userID string) error {
	query := `DELETE FROM group_members WHERE group_id = $1 AND user_id = $2`
	tag, err := r.db.Exec(ctx, query, groupID, userID)
	if err != nil {
		return fmt.Errorf("remove member: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return apperrors.ErrGroupMemberNotFound
	}
	return nil
}

func (r *PostgresGroupRepository) CreateInvitation(ctx context.Context, invitation domain.GroupInvitation) (*domain.GroupInvitation, error) {
	query := `
		INSERT INTO group_invitations (id, group_id, invited_user_id, invited_by, role, status, token, expired_at, responded_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING id, group_id, invited_user_id, invited_by, role, status, token, expired_at, responded_at, created_at, updated_at, deleted_at
	`
	created, err := scanInvitation(r.db.QueryRow(ctx, query, invitation.ID, invitation.GroupID, invitation.InvitedUserID, invitation.InvitedBy, invitation.Role, invitation.Status, invitation.Token, invitation.ExpiredAt, invitation.RespondedAt, invitation.CreatedAt, invitation.UpdatedAt))
	if err != nil {
		if isConflict(err) {
			return nil, apperrors.ErrConflict
		}
		return nil, fmt.Errorf("create invitation: %w", err)
	}
	return created, nil
}

func (r *PostgresGroupRepository) GetInvitation(ctx context.Context, id string) (*domain.GroupInvitation, error) {
	query := `
		SELECT id, group_id, invited_user_id, invited_by, role, status, token, expired_at, responded_at, created_at, updated_at, deleted_at
		FROM group_invitations
		WHERE id = $1 AND deleted_at IS NULL
	`
	invitation, err := scanInvitation(r.db.QueryRow(ctx, query, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.ErrInvitationNotFound
		}
		return nil, fmt.Errorf("get invitation: %w", err)
	}
	return invitation, nil
}

func (r *PostgresGroupRepository) ListInvitations(ctx context.Context, groupID string) ([]domain.GroupInvitation, error) {
	query := `
		SELECT id, group_id, invited_user_id, invited_by, role, status, token, expired_at, responded_at, created_at, updated_at, deleted_at
		FROM group_invitations
		WHERE group_id = $1 AND deleted_at IS NULL
		ORDER BY created_at DESC
	`
	rows, err := r.db.Query(ctx, query, groupID)
	if err != nil {
		return nil, fmt.Errorf("list invitations: %w", err)
	}
	defer rows.Close()

	invitations := make([]domain.GroupInvitation, 0)
	for rows.Next() {
		invitation, scanErr := scanInvitation(rows)
		if scanErr != nil {
			return nil, fmt.Errorf("scan invitation: %w", scanErr)
		}
		invitations = append(invitations, *invitation)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate invitations: %w", err)
	}
	return invitations, nil
}

func (r *PostgresGroupRepository) UpdateInvitation(ctx context.Context, invitation domain.GroupInvitation) (*domain.GroupInvitation, error) {
	query := `
		UPDATE group_invitations
		SET status = $1,
			responded_at = $2,
			updated_at = $3
		WHERE id = $4 AND deleted_at IS NULL
		RETURNING id, group_id, invited_user_id, invited_by, role, status, token, expired_at, responded_at, created_at, updated_at, deleted_at
	`
	updated, err := scanInvitation(r.db.QueryRow(ctx, query, invitation.Status, invitation.RespondedAt, invitation.UpdatedAt, invitation.ID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.ErrInvitationNotFound
		}
		return nil, fmt.Errorf("update invitation: %w", err)
	}
	return updated, nil
}

func (r *PostgresGroupRepository) listGroupsByQuery(ctx context.Context, query string, args ...any) ([]domain.Group, error) {
	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list groups: %w", err)
	}
	defer rows.Close()

	groups := make([]domain.Group, 0)
	for rows.Next() {
		group, scanErr := scanGroup(rows)
		if scanErr != nil {
			return nil, fmt.Errorf("scan group: %w", scanErr)
		}
		groups = append(groups, *group)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate groups: %w", err)
	}
	return groups, nil
}

func scanGroup(row interface{ Scan(dest ...any) error }) (*domain.Group, error) {
	group := &domain.Group{}
	if err := row.Scan(&group.ID, &group.Name, &group.Description, &group.IsActive, &group.CreatedAt, &group.UpdatedAt, &group.DeletedAt); err != nil {
		return nil, err
	}
	return group, nil
}

func scanMember(row interface{ Scan(dest ...any) error }) (*domain.GroupMember, error) {
	member := &domain.GroupMember{}
	if err := row.Scan(&member.GroupID, &member.UserID, &member.Role, &member.JoinedAt, &member.CreatedAt, &member.UpdatedAt); err != nil {
		return nil, err
	}
	return member, nil
}

func scanInvitation(row interface{ Scan(dest ...any) error }) (*domain.GroupInvitation, error) {
	invitation := &domain.GroupInvitation{}
	if err := row.Scan(
		&invitation.ID,
		&invitation.GroupID,
		&invitation.InvitedUserID,
		&invitation.InvitedBy,
		&invitation.Role,
		&invitation.Status,
		&invitation.Token,
		&invitation.ExpiredAt,
		&invitation.RespondedAt,
		&invitation.CreatedAt,
		&invitation.UpdatedAt,
		&invitation.DeletedAt,
	); err != nil {
		return nil, err
	}
	return invitation, nil
}

func isConflict(err error) bool {
	return strings.Contains(strings.ToLower(err.Error()), "duplicate key")
}

var _ GroupRepository = (*PostgresGroupRepository)(nil)
var _ dbExecutor = (*pgxpool.Pool)(nil)
