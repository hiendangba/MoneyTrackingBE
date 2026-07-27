package service

import (
	"context"
	"testing"
	"time"

	"transaction-service/internal/clients"
	"transaction-service/internal/domain"
)

func TestTransactionService_CreateGroupTransaction(t *testing.T) {
	t.Parallel()

	alice := domain.UserSnapshot{UserID: "alice", Fullname: "Alice", Email: "alice@example.com"}
	bob := domain.UserSnapshot{UserID: "bob", Fullname: "Bob", Email: "bob@example.com"}
	repo := &fakeRepository{}
	service := NewTransactionService(
		repo,
		&fakeUserDirectory{users: map[string]domain.UserSnapshot{"alice": alice, "bob": bob}},
		&fakeGroupDirectory{
			group:   clients.GroupSnapshot{ID: "group-1", Name: "Trip", IsActive: true},
			members: []string{"alice", "bob"},
			roles:   map[string]string{"alice": "owner", "bob": "member"},
		},
		"VND",
	)
	fixedNow := time.Date(2026, 7, 27, 12, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return fixedNow }

	created, err := service.CreateGroupTransaction(context.Background(), CreateGroupInput{
		GroupID: "group-1", Type: domain.TransactionTypeExpense, Title: "Lunch",
		Amount: "10.00", Currency: "VND", CreatedBy: "alice",
		Payments: []PaymentInput{{UserID: "alice", Amount: "10.00"}},
		Splits: []SplitInput{
			{UserID: "alice", SplitType: domain.SplitTypeRatio, SplitValue: "1"},
			{UserID: "bob", SplitType: domain.SplitTypeRatio, SplitValue: "1"},
		},
	})
	if err != nil {
		t.Fatalf("CreateGroupTransaction() unexpected error: %v", err)
	}
	if created.GroupName != "Trip" || created.CreatedBy != alice {
		t.Errorf("snapshots not captured: %+v", created)
	}
	if len(created.Splits) != 2 || created.Splits[0].ShareMinor != 500 || created.Splits[1].ShareMinor != 500 {
		t.Errorf("unexpected shares: %+v", created.Splits)
	}
	if repo.createdGroup == nil || repo.createdGroup.AmountMinor != 1000 {
		t.Errorf("repository did not receive aggregate: %+v", repo.createdGroup)
	}
}

func TestTransactionService_CreateGroupTransactionRejectsNonMember(t *testing.T) {
	t.Parallel()

	service := NewTransactionService(
		&fakeRepository{},
		&fakeUserDirectory{users: map[string]domain.UserSnapshot{}},
		&fakeGroupDirectory{
			group:   clients.GroupSnapshot{ID: "group-1", Name: "Trip", IsActive: true},
			members: []string{"alice"},
			roles:   map[string]string{"alice": "owner"},
		},
		"VND",
	)
	_, err := service.CreateGroupTransaction(context.Background(), CreateGroupInput{
		GroupID: "group-1", Type: domain.TransactionTypeExpense, Title: "Lunch",
		Amount: "10.00", CreatedBy: "alice",
		Payments: []PaymentInput{{UserID: "alice", Amount: "10.00"}},
		Splits:   []SplitInput{{UserID: "mallory", SplitType: domain.SplitTypeRatio, SplitValue: "1"}},
	})
	if err == nil {
		t.Fatal("CreateGroupTransaction() error = nil, want non-member validation error")
	}
}

type fakeUserDirectory struct {
	users map[string]domain.UserSnapshot
}

func (f *fakeUserDirectory) GetUsers(_ context.Context, userIDs []string) (map[string]domain.UserSnapshot, error) {
	result := make(map[string]domain.UserSnapshot, len(userIDs))
	for _, userID := range userIDs {
		result[userID] = f.users[userID]
	}
	return result, nil
}

type fakeGroupDirectory struct {
	group   clients.GroupSnapshot
	members []string
	roles   map[string]string
}

func (f *fakeGroupDirectory) GetGroup(context.Context, string) (clients.GroupSnapshot, error) {
	return f.group, nil
}

func (f *fakeGroupDirectory) ListMemberIDs(context.Context, string) ([]string, error) {
	return append([]string(nil), f.members...), nil
}

func (f *fakeGroupDirectory) GetMemberRole(_ context.Context, _, userID string) (string, error) {
	return f.roles[userID], nil
}

type fakeRepository struct {
	createdGroup *domain.GroupTransaction
}

func (f *fakeRepository) CreateCategory(context.Context, domain.Category) (*domain.Category, error) {
	panic("not used")
}

func (f *fakeRepository) GetCategory(context.Context, string) (*domain.Category, error) {
	panic("not used")
}

func (f *fakeRepository) ListCategories(context.Context, bool) ([]domain.Category, error) {
	panic("not used")
}

func (f *fakeRepository) CreatePersonalTransaction(context.Context, domain.PersonalTransaction) (*domain.PersonalTransaction, error) {
	panic("not used")
}

func (f *fakeRepository) GetPersonalTransaction(context.Context, string, string) (*domain.PersonalTransaction, error) {
	panic("not used")
}

func (f *fakeRepository) ListPersonalTransactions(context.Context, string, domain.ListFilter) ([]domain.PersonalTransaction, int64, error) {
	panic("not used")
}

func (f *fakeRepository) DeletePersonalTransaction(context.Context, string, string) error {
	panic("not used")
}

func (f *fakeRepository) CreateGroupTransaction(_ context.Context, transaction domain.GroupTransaction) (*domain.GroupTransaction, error) {
	f.createdGroup = &transaction
	return &transaction, nil
}

func (f *fakeRepository) GetGroupTransaction(context.Context, string) (*domain.GroupTransaction, error) {
	panic("not used")
}

func (f *fakeRepository) ListGroupTransactions(context.Context, string, domain.ListFilter) ([]domain.GroupTransaction, int64, error) {
	panic("not used")
}

func (f *fakeRepository) DeleteGroupTransaction(context.Context, string) error {
	panic("not used")
}

func (f *fakeRepository) CreateSettlement(context.Context, domain.Settlement) (*domain.Settlement, error) {
	panic("not used")
}

func (f *fakeRepository) ListSettlements(context.Context, string, int, int) ([]domain.Settlement, int64, error) {
	panic("not used")
}

func (f *fakeRepository) GetGroupLedger(context.Context, string, string) ([]domain.GroupTransaction, []domain.Settlement, error) {
	panic("not used")
}
