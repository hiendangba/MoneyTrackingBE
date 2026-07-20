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
		SELECT id, fullname, email, password_hash, role_id, is_active, created_at, updated_at, deleted_at
		FROM users
		WHERE email = $1 AND deleted_at IS NULL
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
		SELECT id, fullname, email, password_hash, role_id, is_active, created_at, updated_at, deleted_at
		FROM users
		WHERE id = $1 AND deleted_at IS NULL
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
	query := `SELECT id FROM roles WHERE code = $1 AND deleted_at IS NULL`
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
		INSERT INTO users (id, fullname, email, password_hash, role_id, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, fullname, email, password_hash, role_id, is_active, created_at, updated_at, deleted_at
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

func (r *PostgresUserRepository) UpdatePassword(ctx context.Context, userID string, passwordHash string) error {
	query := `UPDATE users SET password_hash = $1, updated_at = NOW() WHERE id = $2 AND deleted_at IS NULL`
	commandTag, err := r.db.Exec(ctx, query, passwordHash, userID)
	if err != nil {
		return fmt.Errorf("update password: %w", err)
	}
	if commandTag.RowsAffected() == 0 {
		return apperrors.ErrUserNotFound
	}
	return nil
}

func scanUser(_ context.Context, row pgx.Row) (*domain.User, error) {
	user := &domain.User{}
	if err := row.Scan(
		&user.ID,
		&user.Fullname,
		&user.Email,
		&user.PasswordHash,
		&user.RoleID,
		&user.IsActive,
		&user.CreatedAt,
		&user.UpdatedAt,
		&user.DeletedAt,
	); err != nil {
		return nil, err
	}
	return user, nil
}
