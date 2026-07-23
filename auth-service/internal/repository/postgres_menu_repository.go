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

type PostgresMenuRepository struct {
	db *pgxpool.Pool
}

func NewPostgresMenuRepository(db *pgxpool.Pool) *PostgresMenuRepository {
	return &PostgresMenuRepository{db: db}
}

func (r *PostgresMenuRepository) Create(ctx context.Context, menu domain.Menu) (*domain.Menu, error) {
	query := `
		INSERT INTO menus (id, code, name, route, icon, parent_id, sort_order, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id, code, name, route, icon, parent_id, sort_order, is_active, created_at, updated_at, deleted_at
	`

	created, err := scanMenu(r.db.QueryRow(
		ctx,
		query,
		menu.ID,
		menu.Code,
		menu.Name,
		dbRouteValue(menu.Route),
		menu.Icon,
		menu.ParentID,
		menu.SortOrder,
		menu.IsActive,
		menu.CreatedAt,
		menu.UpdatedAt,
	))
	if err != nil {
		if isUniqueViolation(err) {
			return nil, apperrors.ErrConflict
		}
		return nil, fmt.Errorf("create menu: %w", err)
	}
	return created, nil
}

func (r *PostgresMenuRepository) List(ctx context.Context) ([]domain.Menu, error) {
	query := `
		SELECT id, code, name, route, icon, parent_id, sort_order, is_active, created_at, updated_at, deleted_at
		FROM menus
		WHERE deleted_at IS NULL
		ORDER BY sort_order ASC, created_at ASC
	`
	return r.listByQuery(ctx, query)
}

func (r *PostgresMenuRepository) ListActive(ctx context.Context) ([]domain.Menu, error) {
	query := `
		SELECT id, code, name, route, icon, parent_id, sort_order, is_active, created_at, updated_at, deleted_at
		FROM menus
		WHERE deleted_at IS NULL AND is_active = TRUE
		ORDER BY sort_order ASC, created_at ASC
	`
	return r.listByQuery(ctx, query)
}

func (r *PostgresMenuRepository) FindByID(ctx context.Context, id string) (*domain.Menu, error) {
	query := `
		SELECT id, code, name, route, icon, parent_id, sort_order, is_active, created_at, updated_at, deleted_at
		FROM menus
		WHERE id = $1 AND deleted_at IS NULL
	`
	menu, err := scanMenu(r.db.QueryRow(ctx, query, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.ErrMenuNotFound
		}
		return nil, fmt.Errorf("find menu by id: %w", err)
	}
	return menu, nil
}

func (r *PostgresMenuRepository) FindByCode(ctx context.Context, code string) (*domain.Menu, error) {
	query := `
		SELECT id, code, name, route, icon, parent_id, sort_order, is_active, created_at, updated_at, deleted_at
		FROM menus
		WHERE code = $1 AND deleted_at IS NULL
	`
	menu, err := scanMenu(r.db.QueryRow(ctx, query, code))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.ErrMenuNotFound
		}
		return nil, fmt.Errorf("find menu by code: %w", err)
	}
	return menu, nil
}

func (r *PostgresMenuRepository) FindByRoute(ctx context.Context, route string) (*domain.Menu, error) {
	route = strings.TrimSpace(route)
	if route == "" {
		return nil, apperrors.ErrMenuNotFound
	}

	query := `
		SELECT id, code, name, route, icon, parent_id, sort_order, is_active, created_at, updated_at, deleted_at
		FROM menus
		WHERE route = $1 AND deleted_at IS NULL
	`
	menu, err := scanMenu(r.db.QueryRow(ctx, query, route))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.ErrMenuNotFound
		}
		return nil, fmt.Errorf("find menu by route: %w", err)
	}
	return menu, nil
}

func (r *PostgresMenuRepository) Update(ctx context.Context, menu domain.Menu) (*domain.Menu, error) {
	query := `
		UPDATE menus
		SET code = $1,
			name = $2,
			route = $3,
			icon = $4,
			parent_id = $5,
			sort_order = $6,
			is_active = $7,
			updated_at = $8
		WHERE id = $9 AND deleted_at IS NULL
		RETURNING id, code, name, route, icon, parent_id, sort_order, is_active, created_at, updated_at, deleted_at
	`

	updated, err := scanMenu(r.db.QueryRow(
		ctx,
		query,
		menu.Code,
		menu.Name,
		dbRouteValue(menu.Route),
		menu.Icon,
		menu.ParentID,
		menu.SortOrder,
		menu.IsActive,
		menu.UpdatedAt,
		menu.ID,
	))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.ErrMenuNotFound
		}
		if isUniqueViolation(err) {
			return nil, apperrors.ErrConflict
		}
		return nil, fmt.Errorf("update menu: %w", err)
	}
	return updated, nil
}

func (r *PostgresMenuRepository) HasChildren(ctx context.Context, id string) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM menus WHERE parent_id = $1 AND deleted_at IS NULL)`
	var exists bool
	if err := r.db.QueryRow(ctx, query, id).Scan(&exists); err != nil {
		return false, fmt.Errorf("check menu children: %w", err)
	}
	return exists, nil
}

func (r *PostgresMenuRepository) SoftDelete(ctx context.Context, id string) error {
	query := `UPDATE menus SET deleted_at = NOW(), updated_at = NOW() WHERE id = $1 AND deleted_at IS NULL`
	result, err := r.db.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("soft delete menu: %w", err)
	}
	if result.RowsAffected() == 0 {
		return apperrors.ErrMenuNotFound
	}
	return nil
}

func (r *PostgresMenuRepository) listByQuery(ctx context.Context, query string) ([]domain.Menu, error) {
	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query menus: %w", err)
	}
	defer rows.Close()

	menus := make([]domain.Menu, 0)
	for rows.Next() {
		menu, scanErr := scanMenu(rows)
		if scanErr != nil {
			return nil, fmt.Errorf("scan menu row: %w", scanErr)
		}
		menus = append(menus, *menu)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate menus: %w", err)
	}
	return menus, nil
}

func scanMenu(row interface{ Scan(dest ...any) error }) (*domain.Menu, error) {
	menu := &domain.Menu{}
	var route *string
	if err := row.Scan(
		&menu.ID,
		&menu.Code,
		&menu.Name,
		&route,
		&menu.Icon,
		&menu.ParentID,
		&menu.SortOrder,
		&menu.IsActive,
		&menu.CreatedAt,
		&menu.UpdatedAt,
		&menu.DeletedAt,
	); err != nil {
		return nil, err
	}
	if route != nil {
		menu.Route = *route
	}
	return menu, nil
}

func isUniqueViolation(err error) bool {
	return strings.Contains(strings.ToLower(err.Error()), "duplicate key")
}

func dbRouteValue(route string) any {
	route = strings.TrimSpace(route)
	if route == "" {
		return nil
	}
	return route
}
