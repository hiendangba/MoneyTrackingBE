package repository

import (
	"context"

	"github.com/hiendangba/MoneyTrackingBE/Backend/auth-service/internal/domain"
)

type UserRepository interface {
	FindByEmail(ctx context.Context, email string) (*domain.User, error)
	FindByID(ctx context.Context, id string) (*domain.User, error)
	FindRoleIDByCode(ctx context.Context, code string) (string, error)
	Create(ctx context.Context, user domain.User) (*domain.User, error)
	UpdatePasswordAndIncrementSessionVersion(ctx context.Context, userID string, passwordHash string) (int64, error)
}
