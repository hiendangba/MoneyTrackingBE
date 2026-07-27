package repository

import (
	"context"

	"github.com/hiendangba/MoneyTrackingBE/Backend/auth-service/internal/domain"
)

type MenuRepository interface {
	Create(ctx context.Context, menu domain.Menu) (*domain.Menu, error)
	List(ctx context.Context) ([]domain.Menu, error)
	ListActive(ctx context.Context) ([]domain.Menu, error)
	FindByID(ctx context.Context, id string) (*domain.Menu, error)
	FindByCode(ctx context.Context, code string) (*domain.Menu, error)
	FindByRoute(ctx context.Context, route string) (*domain.Menu, error)
	Update(ctx context.Context, menu domain.Menu) (*domain.Menu, error)
	HasChildren(ctx context.Context, id string) (bool, error)
	SoftDelete(ctx context.Context, id string) error
}
