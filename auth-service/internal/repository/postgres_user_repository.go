package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"auth-service/internal/domain"
	apperrors "auth-service/internal/errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresUserRepository struct {
	db *pgxpool.Pool
}

func NewPostgresUserRepository(db *pgxpool.Pool) *PostgresUserRepository {
	return &PostgresUserRepository{db: db}
}

func (r *PostgresUserRepository) FindByEmail(ctx context.Context, email string) (*domain.User, error) {
	query := `
		SELECT u.id, u.fullname, u.email, u.password_hash, u.role_id, r.code, r.is_active, u.session_version, u.is_active, u.created_at, u.updated_at, u.deleted_at
		FROM users u
		JOIN roles r ON r.id = u.role_id
		WHERE u.email = $1 AND u.deleted_at IS NULL
	`

	user, err := scanUser(ctx, r.db.QueryRow(ctx, query, strings.ToLower(email)))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.ErrUserNotFound
		}
		return nil, fmt.Errorf("find user by email: %w", err)
	}

	return user, nil
}

func (r *PostgresUserRepository) FindByID(ctx context.Context, id string) (*domain.User, error) {
	query := `
		SELECT u.id, u.fullname, u.email, u.password_hash, u.role_id, r.code, r.is_active, u.session_version, u.is_active, u.created_at, u.updated_at, u.deleted_at
		FROM users u
		JOIN roles r ON r.id = u.role_id
		WHERE u.id = $1 AND u.deleted_at IS NULL
	`

	user, err := scanUser(ctx, r.db.QueryRow(ctx, query, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.ErrUserNotFound
		}
		return nil, fmt.Errorf("find user by id: %w", err)
	}

	return user, nil
}

func (r *PostgresUserRepository) FindRoleIDByCode(ctx context.Context, code string) (string, error) {
	query := `SELECT id FROM roles WHERE code = $1 AND is_active = TRUE AND deleted_at IS NULL`
	var roleID string
	if err := r.db.QueryRow(ctx, query, code).Scan(&roleID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", apperrors.ErrUserNotFound
		}
		return "", fmt.Errorf("find role by code: %w", err)
	}
	return roleID, nil
}

func (r *PostgresUserRepository) Create(ctx context.Context, user domain.User) (*domain.User, error) {
	query := `
		WITH inserted AS (
			INSERT INTO users (id, fullname, email, password_hash, role_id, is_active, created_at, updated_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			RETURNING id, fullname, email, password_hash, role_id, session_version, is_active, created_at, updated_at, deleted_at
		)
		SELECT i.id, i.fullname, i.email, i.password_hash, i.role_id, r.code, r.is_active, i.session_version, i.is_active, i.created_at, i.updated_at, i.deleted_at
		FROM inserted i
		JOIN roles r ON r.id = i.role_id
	`

	created, err := scanUser(ctx, r.db.QueryRow(
		ctx,
		query,
		user.ID,
		user.Fullname,
		strings.ToLower(user.Email),
		user.PasswordHash,
		user.RoleID,
		user.IsActive,
		user.CreatedAt,
		user.UpdatedAt,
	))
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "duplicate key") {
			return nil, apperrors.ErrEmailAlreadyExists
		}
		return nil, fmt.Errorf("create user: %w", err)
	}

	return created, nil
}

func (r *PostgresUserRepository) UpdatePasswordAndIncrementSessionVersion(ctx context.Context, userID string, passwordHash string) (int64, error) {
	query := `
		UPDATE users
		SET password_hash = $1, session_version = session_version + 1, updated_at = NOW()
		WHERE id = $2 AND deleted_at IS NULL
		RETURNING session_version
	`
	var sessionVersion int64
	if err := r.db.QueryRow(ctx, query, passwordHash, userID).Scan(&sessionVersion); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, apperrors.ErrUserNotFound
		}
		return 0, fmt.Errorf("update password and session version: %w", err)
	}
	return sessionVersion, nil
}

func scanUser(_ context.Context, row pgx.Row) (*domain.User, error) {
	user := &domain.User{}
	if err := row.Scan(
		&user.ID,
		&user.Fullname,
		&user.Email,
		&user.PasswordHash,
		&user.RoleID,
		&user.RoleCode,
		&user.RoleActive,
		&user.SessionVersion,
		&user.IsActive,
		&user.CreatedAt,
		&user.UpdatedAt,
		&user.DeletedAt,
	); err != nil {
		return nil, err
	}
	return user, nil
}
