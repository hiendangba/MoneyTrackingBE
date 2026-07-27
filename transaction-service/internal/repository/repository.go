package repository

import (
	"context"

	"transaction-service/internal/domain"
)

type Repository interface {
	CreateCategory(ctx context.Context, category domain.Category) (*domain.Category, error)
	GetCategory(ctx context.Context, id string) (*domain.Category, error)
	ListCategories(ctx context.Context, includeInactive bool) ([]domain.Category, error)

	CreatePersonalTransaction(ctx context.Context, transaction domain.PersonalTransaction) (*domain.PersonalTransaction, error)
	GetPersonalTransaction(ctx context.Context, id, userID string) (*domain.PersonalTransaction, error)
	ListPersonalTransactions(ctx context.Context, userID string, filter domain.ListFilter) ([]domain.PersonalTransaction, int64, error)
	DeletePersonalTransaction(ctx context.Context, id, userID string) error

	CreateGroupTransaction(ctx context.Context, transaction domain.GroupTransaction) (*domain.GroupTransaction, error)
	GetGroupTransaction(ctx context.Context, id string) (*domain.GroupTransaction, error)
	ListGroupTransactions(ctx context.Context, groupID string, filter domain.ListFilter) ([]domain.GroupTransaction, int64, error)
	DeleteGroupTransaction(ctx context.Context, id string) error

	CreateSettlement(ctx context.Context, settlement domain.Settlement) (*domain.Settlement, error)
	ListSettlements(ctx context.Context, groupID string, limit, offset int) ([]domain.Settlement, int64, error)
	GetGroupLedger(ctx context.Context, groupID, currency string) ([]domain.GroupTransaction, []domain.Settlement, error)
}
