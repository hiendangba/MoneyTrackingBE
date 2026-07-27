package service

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/hiendangba/MoneyTrackingBE/Backend/transaction-service/internal/clients"
	"github.com/hiendangba/MoneyTrackingBE/Backend/transaction-service/internal/domain"
	apperrors "github.com/hiendangba/MoneyTrackingBE/Backend/transaction-service/internal/errors"
	"github.com/hiendangba/MoneyTrackingBE/Backend/transaction-service/internal/repository"

	"github.com/google/uuid"
)

type TransactionService struct {
	repo            repository.Repository
	users           clients.UserDirectory
	groups          clients.GroupDirectory
	defaultCurrency string
	now             func() time.Time
}

func NewTransactionService(
	repo repository.Repository,
	users clients.UserDirectory,
	groups clients.GroupDirectory,
	defaultCurrency string,
) *TransactionService {
	return &TransactionService{
		repo:            repo,
		users:           users,
		groups:          groups,
		defaultCurrency: normalizeCurrency(defaultCurrency, "VND"),
		now:             time.Now,
	}
}

func (s *TransactionService) CreateCategory(ctx context.Context, input CreateCategoryInput) (*domain.Category, error) {
	code := strings.ToLower(strings.TrimSpace(input.Code))
	name := strings.TrimSpace(input.Name)
	if code == "" || name == "" {
		return nil, apperrors.Validation("code and name are required")
	}
	id, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("generate category id: %w", err)
	}
	now := s.now()
	return s.repo.CreateCategory(ctx, domain.Category{
		ID:          id.String(),
		Code:        code,
		Name:        name,
		Description: optional(input.Description),
		IsActive:    true,
		CreatedAt:   now,
		UpdatedAt:   now,
	})
}

func (s *TransactionService) ListCategories(ctx context.Context, includeInactive bool) ([]domain.Category, error) {
	return s.repo.ListCategories(ctx, includeInactive)
}

func (s *TransactionService) CreatePersonalTransaction(ctx context.Context, input CreatePersonalInput) (*domain.PersonalTransaction, error) {
	userID := strings.TrimSpace(input.UserID)
	title := strings.TrimSpace(input.Title)
	if userID == "" || title == "" {
		return nil, apperrors.Validation("user_id and title are required")
	}
	if !validTransactionType(input.Type) {
		return nil, apperrors.Validation("transaction type must be income or expense")
	}
	amount, err := positiveMoney(input.Amount)
	if err != nil {
		return nil, err
	}
	currency, err := s.currency(input.Currency)
	if err != nil {
		return nil, err
	}
	profiles, err := s.users.GetUsers(ctx, []string{userID})
	if err != nil {
		return nil, err
	}
	category, err := s.categorySnapshot(ctx, input.CategoryID)
	if err != nil {
		return nil, err
	}
	id, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("generate personal transaction id: %w", err)
	}
	now := s.now()
	transactionDate := input.TransactionDate
	if transactionDate.IsZero() {
		transactionDate = now
	}
	transaction := domain.PersonalTransaction{
		ID:              id.String(),
		User:            profiles[userID],
		Type:            input.Type,
		Title:           title,
		AmountMinor:     amount,
		Currency:        currency,
		TransactionDate: transactionDate,
		Note:            optional(input.Note),
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	applyCategoryToPersonal(&transaction, category)
	return s.repo.CreatePersonalTransaction(ctx, transaction)
}

func (s *TransactionService) GetPersonalTransaction(ctx context.Context, id, userID string) (*domain.PersonalTransaction, error) {
	id = strings.TrimSpace(id)
	userID = strings.TrimSpace(userID)
	if id == "" || userID == "" {
		return nil, apperrors.Validation("id and user_id are required")
	}
	return s.repo.GetPersonalTransaction(ctx, id, userID)
}

func (s *TransactionService) ListPersonalTransactions(
	ctx context.Context,
	userID string,
	filter domain.ListFilter,
) ([]domain.PersonalTransaction, int64, error) {
	if strings.TrimSpace(userID) == "" {
		return nil, 0, apperrors.Validation("user_id is required")
	}
	normalizeFilter(&filter)
	return s.repo.ListPersonalTransactions(ctx, userID, filter)
}

func (s *TransactionService) DeletePersonalTransaction(ctx context.Context, id, userID string) error {
	if strings.TrimSpace(id) == "" || strings.TrimSpace(userID) == "" {
		return apperrors.Validation("id and user_id are required")
	}
	return s.repo.DeletePersonalTransaction(ctx, id, userID)
}

func (s *TransactionService) CreateGroupTransaction(ctx context.Context, input CreateGroupInput) (*domain.GroupTransaction, error) {
	groupID := strings.TrimSpace(input.GroupID)
	createdBy := strings.TrimSpace(input.CreatedBy)
	title := strings.TrimSpace(input.Title)
	if groupID == "" || createdBy == "" || title == "" {
		return nil, apperrors.Validation("group_id, created_by and title are required")
	}
	if !validTransactionType(input.Type) {
		return nil, apperrors.Validation("transaction type must be income or expense")
	}
	if _, err := s.groups.GetMemberRole(ctx, groupID, createdBy); err != nil {
		return nil, err
	}
	group, err := s.groups.GetGroup(ctx, groupID)
	if err != nil {
		return nil, err
	}
	if !group.IsActive {
		return nil, apperrors.Validation("group is inactive")
	}
	amount, err := positiveMoney(input.Amount)
	if err != nil {
		return nil, err
	}
	currency, err := s.currency(input.Currency)
	if err != nil {
		return nil, err
	}
	memberIDs, err := s.groups.ListMemberIDs(ctx, groupID)
	if err != nil {
		return nil, err
	}
	memberSet := make(map[string]struct{}, len(memberIDs))
	for _, memberID := range memberIDs {
		memberSet[memberID] = struct{}{}
	}
	userIDs := []string{createdBy}
	for _, payment := range input.Payments {
		userIDs = append(userIDs, strings.TrimSpace(payment.UserID))
	}
	for _, split := range input.Splits {
		userIDs = append(userIDs, strings.TrimSpace(split.UserID))
	}
	for _, userID := range userIDs {
		if _, exists := memberSet[userID]; !exists {
			return nil, apperrors.Validation("all payers and participants must be active group members")
		}
	}
	profiles, err := s.users.GetUsers(ctx, userIDs)
	if err != nil {
		return nil, err
	}
	category, err := s.categorySnapshot(ctx, input.CategoryID)
	if err != nil {
		return nil, err
	}
	now := s.now()
	payments, err := buildPayments(input.Payments, profiles, now)
	if err != nil {
		return nil, err
	}
	if err := domain.ValidatePayments(amount, payments); err != nil {
		return nil, fmt.Errorf("%w: %v", apperrors.ErrInvalidPayment, err)
	}
	splits, err := buildSplits(input.Splits, profiles, amount, now)
	if err != nil {
		return nil, err
	}
	id, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("generate group transaction id: %w", err)
	}
	for i := range payments {
		payments[i].TransactionID = id.String()
	}
	for i := range splits {
		splits[i].TransactionID = id.String()
	}
	transactionDate := input.TransactionDate
	if transactionDate.IsZero() {
		transactionDate = now
	}
	transaction := domain.GroupTransaction{
		ID:              id.String(),
		GroupID:         group.ID,
		GroupName:       group.Name,
		Type:            input.Type,
		Title:           title,
		AmountMinor:     amount,
		Currency:        currency,
		TransactionDate: transactionDate,
		Note:            optional(input.Note),
		CreatedBy:       profiles[createdBy],
		Payments:        payments,
		Splits:          splits,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	applyCategoryToGroup(&transaction, category)
	return s.repo.CreateGroupTransaction(ctx, transaction)
}

func (s *TransactionService) GetGroupTransaction(ctx context.Context, id, userID string) (*domain.GroupTransaction, error) {
	if strings.TrimSpace(id) == "" || strings.TrimSpace(userID) == "" {
		return nil, apperrors.Validation("id and user_id are required")
	}
	transaction, err := s.repo.GetGroupTransaction(ctx, id)
	if err != nil {
		return nil, err
	}
	if _, err := s.groups.GetMemberRole(ctx, transaction.GroupID, userID); err != nil {
		return nil, err
	}
	return transaction, nil
}

func (s *TransactionService) ListGroupTransactions(
	ctx context.Context,
	groupID, userID string,
	filter domain.ListFilter,
) ([]domain.GroupTransaction, int64, error) {
	if strings.TrimSpace(groupID) == "" || strings.TrimSpace(userID) == "" {
		return nil, 0, apperrors.Validation("group_id and user_id are required")
	}
	if _, err := s.groups.GetMemberRole(ctx, groupID, userID); err != nil {
		return nil, 0, err
	}
	normalizeFilter(&filter)
	return s.repo.ListGroupTransactions(ctx, groupID, filter)
}

func (s *TransactionService) DeleteGroupTransaction(ctx context.Context, id, userID string) error {
	transaction, err := s.GetGroupTransaction(ctx, id, userID)
	if err != nil {
		return err
	}
	role, err := s.groups.GetMemberRole(ctx, transaction.GroupID, userID)
	if err != nil {
		return err
	}
	if transaction.CreatedBy.UserID != userID && role != "owner" {
		return apperrors.ErrForbidden
	}
	return s.repo.DeleteGroupTransaction(ctx, id)
}

func (s *TransactionService) CreateSettlement(ctx context.Context, input CreateSettlementInput) (*domain.Settlement, error) {
	groupID := strings.TrimSpace(input.GroupID)
	fromUserID := strings.TrimSpace(input.FromUserID)
	toUserID := strings.TrimSpace(input.ToUserID)
	createdBy := strings.TrimSpace(input.CreatedBy)
	if groupID == "" || fromUserID == "" || toUserID == "" || createdBy == "" {
		return nil, apperrors.Validation("group_id, from_user_id, to_user_id and created_by are required")
	}
	if fromUserID == toUserID {
		return nil, apperrors.Validation("from_user_id and to_user_id must differ")
	}
	role, err := s.groups.GetMemberRole(ctx, groupID, createdBy)
	if err != nil {
		return nil, err
	}
	if createdBy != fromUserID && role != "owner" {
		return nil, apperrors.ErrForbidden
	}
	group, err := s.groups.GetGroup(ctx, groupID)
	if err != nil {
		return nil, err
	}
	memberIDs, err := s.groups.ListMemberIDs(ctx, groupID)
	if err != nil {
		return nil, err
	}
	if !contains(memberIDs, fromUserID) || !contains(memberIDs, toUserID) {
		return nil, apperrors.Validation("settlement users must be active group members")
	}
	profiles, err := s.users.GetUsers(ctx, []string{fromUserID, toUserID, createdBy})
	if err != nil {
		return nil, err
	}
	amount, err := positiveMoney(input.Amount)
	if err != nil {
		return nil, err
	}
	currency, err := s.currency(input.Currency)
	if err != nil {
		return nil, err
	}
	id, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("generate settlement id: %w", err)
	}
	now := s.now()
	settledAt := input.SettledAt
	if settledAt.IsZero() {
		settledAt = now
	}
	return s.repo.CreateSettlement(ctx, domain.Settlement{
		ID:          id.String(),
		GroupID:     group.ID,
		GroupName:   group.Name,
		FromUser:    profiles[fromUserID],
		ToUser:      profiles[toUserID],
		AmountMinor: amount,
		Currency:    currency,
		SettledAt:   settledAt,
		Note:        optional(input.Note),
		CreatedBy:   profiles[createdBy],
		CreatedAt:   now,
		UpdatedAt:   now,
	})
}

func (s *TransactionService) ListSettlements(
	ctx context.Context,
	groupID, userID string,
	limit, offset int,
) ([]domain.Settlement, int64, error) {
	if strings.TrimSpace(groupID) == "" || strings.TrimSpace(userID) == "" {
		return nil, 0, apperrors.Validation("group_id and user_id are required")
	}
	if _, err := s.groups.GetMemberRole(ctx, groupID, userID); err != nil {
		return nil, 0, err
	}
	limit, offset = normalizePage(limit, offset)
	return s.repo.ListSettlements(ctx, groupID, limit, offset)
}

func (s *TransactionService) GetGroupBalances(
	ctx context.Context,
	groupID, userID, currency string,
) ([]domain.MemberBalance, []domain.SettlementSuggestion, error) {
	if strings.TrimSpace(groupID) == "" || strings.TrimSpace(userID) == "" {
		return nil, nil, apperrors.Validation("group_id and user_id are required")
	}
	if _, err := s.groups.GetMemberRole(ctx, groupID, userID); err != nil {
		return nil, nil, err
	}
	currency, err := s.currency(currency)
	if err != nil {
		return nil, nil, err
	}
	transactions, settlements, err := s.repo.GetGroupLedger(ctx, groupID, currency)
	if err != nil {
		return nil, nil, err
	}
	allBalances := make(map[string]*domain.MemberBalance)
	for _, transaction := range transactions {
		balances, calcErr := domain.CalculateBalances(transaction.Type, transaction.Payments, transaction.Splits, nil)
		if calcErr != nil {
			return nil, nil, fmt.Errorf("calculate transaction %s: %w", transaction.ID, calcErr)
		}
		mergeBalances(allBalances, balances)
	}
	settlementBalances, err := domain.CalculateBalances("", nil, nil, settlements)
	if err != nil && !errors.Is(err, domain.ErrUnbalancedNet) {
		return nil, nil, fmt.Errorf("calculate settlements: %w", err)
	}
	mergeBalances(allBalances, settlementBalances)
	balances := make([]domain.MemberBalance, 0, len(allBalances))
	var total int64
	for _, balance := range allBalances {
		total += balance.NetMinor
		balances = append(balances, *balance)
	}
	if total != 0 {
		return nil, nil, domain.ErrUnbalancedNet
	}
	sort.Slice(balances, func(i, j int) bool {
		return balances[i].User.UserID < balances[j].User.UserID
	})
	return balances, domain.SuggestSettlements(balances, currency), nil
}

func buildPayments(
	inputs []PaymentInput,
	profiles map[string]domain.UserSnapshot,
	now time.Time,
) ([]domain.GroupPayment, error) {
	result := make([]domain.GroupPayment, 0, len(inputs))
	for _, input := range inputs {
		amount, err := positiveMoney(input.Amount)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", apperrors.ErrInvalidPayment, err)
		}
		id, err := uuid.NewV7()
		if err != nil {
			return nil, fmt.Errorf("generate payment id: %w", err)
		}
		userID := strings.TrimSpace(input.UserID)
		result = append(result, domain.GroupPayment{
			ID:          id.String(),
			User:        profiles[userID],
			AmountMinor: amount,
			Note:        optional(input.Note),
			CreatedAt:   now,
			UpdatedAt:   now,
		})
	}
	return result, nil
}

func buildSplits(
	inputs []SplitInput,
	profiles map[string]domain.UserSnapshot,
	amount int64,
	now time.Time,
) ([]domain.GroupSplit, error) {
	allocations := make([]domain.SplitAllocation, 0, len(inputs))
	for _, input := range inputs {
		var value int64
		var err error
		switch input.SplitType {
		case domain.SplitTypeFixed:
			value, err = positiveMoney(input.SplitValue)
		case domain.SplitTypeRatio:
			value, err = domain.ParseRatio(input.SplitValue)
			if err == nil && value <= 0 {
				err = domain.ErrInvalidSplitValue
			}
		default:
			err = domain.ErrInvalidSplitValue
		}
		if err != nil {
			return nil, fmt.Errorf("%w: %v", apperrors.ErrInvalidSplit, err)
		}
		allocations = append(allocations, domain.SplitAllocation{
			UserID:     strings.TrimSpace(input.UserID),
			SplitType:  input.SplitType,
			SplitValue: value,
		})
	}
	allocated, err := domain.AllocateShares(amount, allocations)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", apperrors.ErrInvalidSplit, err)
	}
	result := make([]domain.GroupSplit, 0, len(allocated))
	for _, allocation := range allocated {
		id, err := uuid.NewV7()
		if err != nil {
			return nil, fmt.Errorf("generate split id: %w", err)
		}
		result = append(result, domain.GroupSplit{
			ID:         id.String(),
			User:       profiles[allocation.UserID],
			SplitType:  allocation.SplitType,
			SplitValue: allocation.SplitValue,
			ShareMinor: allocation.ShareMinor,
			CreatedAt:  now,
			UpdatedAt:  now,
		})
	}
	return result, nil
}

func (s *TransactionService) categorySnapshot(ctx context.Context, categoryID string) (*domain.Category, error) {
	categoryID = strings.TrimSpace(categoryID)
	if categoryID == "" {
		return nil, nil
	}
	category, err := s.repo.GetCategory(ctx, categoryID)
	if err != nil {
		return nil, err
	}
	if !category.IsActive {
		return nil, apperrors.Validation("category is inactive")
	}
	return category, nil
}

func positiveMoney(raw string) (int64, error) {
	amount, err := domain.ParseMoney(raw)
	if err != nil || amount <= 0 {
		return 0, apperrors.Validation("amount must be a positive decimal with at most 2 fractional digits")
	}
	return amount, nil
}

func validTransactionType(value domain.TransactionType) bool {
	return value == domain.TransactionTypeIncome || value == domain.TransactionTypeExpense
}

func normalizeCurrency(value, fallback string) string {
	value = strings.ToUpper(strings.TrimSpace(value))
	if value == "" {
		value = strings.ToUpper(strings.TrimSpace(fallback))
	}
	if len(value) != 3 {
		return strings.ToUpper(strings.TrimSpace(fallback))
	}
	return value
}

func (s *TransactionService) currency(value string) (string, error) {
	value = strings.ToUpper(strings.TrimSpace(value))
	if value == "" {
		value = s.defaultCurrency
	}
	if len(value) != 3 {
		return "", apperrors.Validation("currency must be a 3-letter code")
	}
	for _, character := range value {
		if character < 'A' || character > 'Z' {
			return "", apperrors.Validation("currency must contain only letters")
		}
	}
	return value, nil
}

func normalizeFilter(filter *domain.ListFilter) {
	filter.Limit, filter.Offset = normalizePage(filter.Limit, filter.Offset)
	if !validTransactionType(filter.Type) {
		filter.Type = ""
	}
	filter.CategoryID = strings.TrimSpace(filter.CategoryID)
}

func normalizePage(limit, offset int) (int, int) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	if offset < 0 {
		offset = 0
	}
	return limit, offset
}

func optional(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return &value
}

func applyCategoryToPersonal(transaction *domain.PersonalTransaction, category *domain.Category) {
	if category == nil {
		return
	}
	transaction.CategoryID = &category.ID
	transaction.CategoryCode = &category.Code
	transaction.CategoryName = &category.Name
}

func applyCategoryToGroup(transaction *domain.GroupTransaction, category *domain.Category) {
	if category == nil {
		return
	}
	transaction.CategoryID = &category.ID
	transaction.CategoryCode = &category.Code
	transaction.CategoryName = &category.Name
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func mergeBalances(target map[string]*domain.MemberBalance, source []domain.MemberBalance) {
	for _, balance := range source {
		current, exists := target[balance.User.UserID]
		if !exists {
			copy := balance
			target[balance.User.UserID] = &copy
			continue
		}
		current.PaidMinor += balance.PaidMinor
		current.ShareMinor += balance.ShareMinor
		current.SettlementsSent += balance.SettlementsSent
		current.SettlementsReceived += balance.SettlementsReceived
		current.NetMinor += balance.NetMinor
	}
}
