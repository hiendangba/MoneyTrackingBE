package grpctransport

import (
	"context"
	"errors"
	"log/slog"
	"time"

	transactionv1 "transaction-service/gen/transaction/v1"
	"transaction-service/internal/domain"
	apperrors "transaction-service/internal/errors"
	"transaction-service/internal/service"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Server struct {
	transactionv1.UnimplementedTransactionServiceServer
	service *service.TransactionService
	logger  *slog.Logger
}

func NewServer(transactionService *service.TransactionService, logger *slog.Logger) *Server {
	return &Server{service: transactionService, logger: logger}
}

func (s *Server) CreateCategory(ctx context.Context, req *transactionv1.CreateCategoryRequest) (*transactionv1.Category, error) {
	category, err := s.service.CreateCategory(ctx, service.CreateCategoryInput{
		Code: req.GetCode(), Name: req.GetName(), Description: req.GetDescription(),
	})
	if err != nil {
		return nil, mapError(err)
	}
	return toProtoCategory(category), nil
}

func (s *Server) ListCategories(ctx context.Context, req *transactionv1.ListCategoriesRequest) (*transactionv1.ListCategoriesResponse, error) {
	categories, err := s.service.ListCategories(ctx, req.GetIncludeInactive())
	if err != nil {
		return nil, mapError(err)
	}
	response := &transactionv1.ListCategoriesResponse{Categories: make([]*transactionv1.Category, 0, len(categories))}
	for i := range categories {
		response.Categories = append(response.Categories, toProtoCategory(&categories[i]))
	}
	return response, nil
}

func (s *Server) CreatePersonalTransaction(ctx context.Context, req *transactionv1.CreatePersonalTransactionRequest) (*transactionv1.PersonalTransaction, error) {
	transaction, err := s.service.CreatePersonalTransaction(ctx, service.CreatePersonalInput{
		UserID: req.GetUserId(), CategoryID: req.GetCategoryId(), Type: typeFromProto(req.GetType()),
		Title: req.GetTitle(), Amount: req.GetAmount(), Currency: req.GetCurrency(),
		TransactionDate: timeFromProto(req.GetTransactionDate()), Note: req.GetNote(),
	})
	if err != nil {
		return nil, mapError(err)
	}
	return toProtoPersonal(transaction), nil
}

func (s *Server) GetPersonalTransaction(ctx context.Context, req *transactionv1.GetPersonalTransactionRequest) (*transactionv1.PersonalTransaction, error) {
	transaction, err := s.service.GetPersonalTransaction(ctx, req.GetId(), req.GetUserId())
	if err != nil {
		return nil, mapError(err)
	}
	return toProtoPersonal(transaction), nil
}

func (s *Server) ListPersonalTransactions(ctx context.Context, req *transactionv1.ListPersonalTransactionsRequest) (*transactionv1.ListPersonalTransactionsResponse, error) {
	transactions, total, err := s.service.ListPersonalTransactions(ctx, req.GetUserId(), filterFromPersonalRequest(req))
	if err != nil {
		return nil, mapError(err)
	}
	response := &transactionv1.ListPersonalTransactionsResponse{
		Transactions: make([]*transactionv1.PersonalTransaction, 0, len(transactions)),
		Total:        total,
	}
	for i := range transactions {
		response.Transactions = append(response.Transactions, toProtoPersonal(&transactions[i]))
	}
	return response, nil
}

func (s *Server) DeletePersonalTransaction(ctx context.Context, req *transactionv1.DeletePersonalTransactionRequest) (*transactionv1.DeleteResponse, error) {
	if err := s.service.DeletePersonalTransaction(ctx, req.GetId(), req.GetUserId()); err != nil {
		return nil, mapError(err)
	}
	return &transactionv1.DeleteResponse{Message: "personal transaction deleted"}, nil
}

func (s *Server) CreateGroupTransaction(ctx context.Context, req *transactionv1.CreateGroupTransactionRequest) (*transactionv1.GroupTransaction, error) {
	payments := make([]service.PaymentInput, 0, len(req.GetPayments()))
	for _, payment := range req.GetPayments() {
		payments = append(payments, service.PaymentInput{
			UserID: payment.GetUserId(), Amount: payment.GetAmount(), Note: payment.GetNote(),
		})
	}
	splits := make([]service.SplitInput, 0, len(req.GetSplits()))
	for _, split := range req.GetSplits() {
		splits = append(splits, service.SplitInput{
			UserID: split.GetUserId(), SplitType: splitTypeFromProto(split.GetSplitType()),
			SplitValue: split.GetSplitValue(),
		})
	}
	transaction, err := s.service.CreateGroupTransaction(ctx, service.CreateGroupInput{
		GroupID: req.GetGroupId(), CategoryID: req.GetCategoryId(), Type: typeFromProto(req.GetType()),
		Title: req.GetTitle(), Amount: req.GetAmount(), Currency: req.GetCurrency(),
		TransactionDate: timeFromProto(req.GetTransactionDate()), Note: req.GetNote(),
		CreatedBy: req.GetCreatedBy(), Payments: payments, Splits: splits,
	})
	if err != nil {
		return nil, mapError(err)
	}
	return toProtoGroupTransaction(transaction), nil
}

func (s *Server) GetGroupTransaction(ctx context.Context, req *transactionv1.GetGroupTransactionRequest) (*transactionv1.GroupTransaction, error) {
	transaction, err := s.service.GetGroupTransaction(ctx, req.GetId(), req.GetUserId())
	if err != nil {
		return nil, mapError(err)
	}
	return toProtoGroupTransaction(transaction), nil
}

func (s *Server) ListGroupTransactions(ctx context.Context, req *transactionv1.ListGroupTransactionsRequest) (*transactionv1.ListGroupTransactionsResponse, error) {
	transactions, total, err := s.service.ListGroupTransactions(ctx, req.GetGroupId(), req.GetUserId(), filterFromGroupRequest(req))
	if err != nil {
		return nil, mapError(err)
	}
	response := &transactionv1.ListGroupTransactionsResponse{
		Transactions: make([]*transactionv1.GroupTransaction, 0, len(transactions)),
		Total:        total,
	}
	for i := range transactions {
		response.Transactions = append(response.Transactions, toProtoGroupTransaction(&transactions[i]))
	}
	return response, nil
}

func (s *Server) DeleteGroupTransaction(ctx context.Context, req *transactionv1.DeleteGroupTransactionRequest) (*transactionv1.DeleteResponse, error) {
	if err := s.service.DeleteGroupTransaction(ctx, req.GetId(), req.GetUserId()); err != nil {
		return nil, mapError(err)
	}
	return &transactionv1.DeleteResponse{Message: "group transaction deleted"}, nil
}

func (s *Server) CreateSettlement(ctx context.Context, req *transactionv1.CreateSettlementRequest) (*transactionv1.Settlement, error) {
	settlement, err := s.service.CreateSettlement(ctx, service.CreateSettlementInput{
		GroupID: req.GetGroupId(), FromUserID: req.GetFromUserId(), ToUserID: req.GetToUserId(),
		Amount: req.GetAmount(), Currency: req.GetCurrency(), SettledAt: timeFromProto(req.GetSettledAt()),
		Note: req.GetNote(), CreatedBy: req.GetCreatedBy(),
	})
	if err != nil {
		return nil, mapError(err)
	}
	return toProtoSettlement(settlement), nil
}

func (s *Server) ListSettlements(ctx context.Context, req *transactionv1.ListSettlementsRequest) (*transactionv1.ListSettlementsResponse, error) {
	settlements, total, err := s.service.ListSettlements(
		ctx, req.GetGroupId(), req.GetUserId(), int(req.GetLimit()), int(req.GetOffset()),
	)
	if err != nil {
		return nil, mapError(err)
	}
	response := &transactionv1.ListSettlementsResponse{
		Settlements: make([]*transactionv1.Settlement, 0, len(settlements)),
		Total:       total,
	}
	for i := range settlements {
		response.Settlements = append(response.Settlements, toProtoSettlement(&settlements[i]))
	}
	return response, nil
}

func (s *Server) GetGroupBalances(ctx context.Context, req *transactionv1.GetGroupBalancesRequest) (*transactionv1.GetGroupBalancesResponse, error) {
	balances, suggestions, err := s.service.GetGroupBalances(
		ctx, req.GetGroupId(), req.GetUserId(), req.GetCurrency(),
	)
	if err != nil {
		return nil, mapError(err)
	}
	response := &transactionv1.GetGroupBalancesResponse{
		Balances:    make([]*transactionv1.MemberBalance, 0, len(balances)),
		Suggestions: make([]*transactionv1.SettlementSuggestion, 0, len(suggestions)),
	}
	for i := range balances {
		response.Balances = append(response.Balances, toProtoBalance(&balances[i]))
	}
	for i := range suggestions {
		response.Suggestions = append(response.Suggestions, toProtoSuggestion(&suggestions[i]))
	}
	return response, nil
}

func toProtoCategory(category *domain.Category) *transactionv1.Category {
	if category == nil {
		return nil
	}
	return &transactionv1.Category{
		Id: category.ID, Code: category.Code, Name: category.Name,
		Description: deref(category.Description), IsActive: category.IsActive,
		CreatedAt: timestamppb.New(category.CreatedAt), UpdatedAt: timestamppb.New(category.UpdatedAt),
	}
}

func toProtoUser(user domain.UserSnapshot) *transactionv1.UserSnapshot {
	return &transactionv1.UserSnapshot{UserId: user.UserID, Fullname: user.Fullname, Email: user.Email}
}

func toProtoPersonal(transaction *domain.PersonalTransaction) *transactionv1.PersonalTransaction {
	if transaction == nil {
		return nil
	}
	return &transactionv1.PersonalTransaction{
		Id: transaction.ID, User: toProtoUser(transaction.User), Category: categoryFromSnapshot(
			transaction.CategoryID, transaction.CategoryCode, transaction.CategoryName,
		), Type: typeToProto(transaction.Type), Title: transaction.Title,
		Amount: domain.FormatMoney(transaction.AmountMinor), Currency: transaction.Currency,
		TransactionDate: timestamppb.New(transaction.TransactionDate), Note: deref(transaction.Note),
		CreatedAt: timestamppb.New(transaction.CreatedAt), UpdatedAt: timestamppb.New(transaction.UpdatedAt),
	}
}

func toProtoGroupTransaction(transaction *domain.GroupTransaction) *transactionv1.GroupTransaction {
	if transaction == nil {
		return nil
	}
	response := &transactionv1.GroupTransaction{
		Id: transaction.ID, GroupId: transaction.GroupID, GroupName: transaction.GroupName,
		Category: categoryFromSnapshot(transaction.CategoryID, transaction.CategoryCode, transaction.CategoryName),
		Type:     typeToProto(transaction.Type), Title: transaction.Title,
		Amount: domain.FormatMoney(transaction.AmountMinor), Currency: transaction.Currency,
		TransactionDate: timestamppb.New(transaction.TransactionDate), Note: deref(transaction.Note),
		CreatedBy: toProtoUser(transaction.CreatedBy),
		Payments:  make([]*transactionv1.GroupPayment, 0, len(transaction.Payments)),
		Splits:    make([]*transactionv1.GroupSplit, 0, len(transaction.Splits)),
		CreatedAt: timestamppb.New(transaction.CreatedAt), UpdatedAt: timestamppb.New(transaction.UpdatedAt),
	}
	for i := range transaction.Payments {
		payment := &transaction.Payments[i]
		response.Payments = append(response.Payments, &transactionv1.GroupPayment{
			Id: payment.ID, User: toProtoUser(payment.User), Amount: domain.FormatMoney(payment.AmountMinor),
			Note: deref(payment.Note), CreatedAt: timestamppb.New(payment.CreatedAt),
		})
	}
	for i := range transaction.Splits {
		split := &transaction.Splits[i]
		value := domain.FormatRatio(split.SplitValue)
		if split.SplitType == domain.SplitTypeFixed {
			value = domain.FormatMoney(split.SplitValue)
		}
		response.Splits = append(response.Splits, &transactionv1.GroupSplit{
			Id: split.ID, User: toProtoUser(split.User), SplitType: splitTypeToProto(split.SplitType),
			SplitValue: value, ShareAmount: domain.FormatMoney(split.ShareMinor),
			CreatedAt: timestamppb.New(split.CreatedAt),
		})
	}
	return response
}

func toProtoSettlement(settlement *domain.Settlement) *transactionv1.Settlement {
	if settlement == nil {
		return nil
	}
	return &transactionv1.Settlement{
		Id: settlement.ID, GroupId: settlement.GroupID, GroupName: settlement.GroupName,
		FromUser: toProtoUser(settlement.FromUser), ToUser: toProtoUser(settlement.ToUser),
		Amount: domain.FormatMoney(settlement.AmountMinor), Currency: settlement.Currency,
		SettledAt: timestamppb.New(settlement.SettledAt), Note: deref(settlement.Note),
		CreatedBy: toProtoUser(settlement.CreatedBy), CreatedAt: timestamppb.New(settlement.CreatedAt),
	}
}

func toProtoBalance(balance *domain.MemberBalance) *transactionv1.MemberBalance {
	return &transactionv1.MemberBalance{
		User: toProtoUser(balance.User), Paid: domain.FormatMoney(balance.PaidMinor),
		Share:               domain.FormatMoney(balance.ShareMinor),
		SettlementsSent:     domain.FormatMoney(balance.SettlementsSent),
		SettlementsReceived: domain.FormatMoney(balance.SettlementsReceived),
		NetBalance:          domain.FormatMoney(balance.NetMinor),
	}
}

func toProtoSuggestion(suggestion *domain.SettlementSuggestion) *transactionv1.SettlementSuggestion {
	return &transactionv1.SettlementSuggestion{
		FromUser: toProtoUser(suggestion.FromUser), ToUser: toProtoUser(suggestion.ToUser),
		Amount: domain.FormatMoney(suggestion.AmountMinor), Currency: suggestion.Currency,
	}
}

func categoryFromSnapshot(id, code, name *string) *transactionv1.Category {
	if id == nil {
		return nil
	}
	return &transactionv1.Category{Id: deref(id), Code: deref(code), Name: deref(name), IsActive: true}
}

func filterFromPersonalRequest(req *transactionv1.ListPersonalTransactionsRequest) domain.ListFilter {
	return domain.ListFilter{
		From: timePtrFromProto(req.GetFrom()), To: timePtrFromProto(req.GetTo()),
		Type: typeFromProto(req.GetType()), CategoryID: req.GetCategoryId(),
		Limit: int(req.GetLimit()), Offset: int(req.GetOffset()),
	}
}

func filterFromGroupRequest(req *transactionv1.ListGroupTransactionsRequest) domain.ListFilter {
	return domain.ListFilter{
		From: timePtrFromProto(req.GetFrom()), To: timePtrFromProto(req.GetTo()),
		Type: typeFromProto(req.GetType()), CategoryID: req.GetCategoryId(),
		Limit: int(req.GetLimit()), Offset: int(req.GetOffset()),
	}
}

func typeFromProto(value transactionv1.TransactionType) domain.TransactionType {
	switch value {
	case transactionv1.TransactionType_TRANSACTION_TYPE_INCOME:
		return domain.TransactionTypeIncome
	case transactionv1.TransactionType_TRANSACTION_TYPE_EXPENSE:
		return domain.TransactionTypeExpense
	default:
		return ""
	}
}

func typeToProto(value domain.TransactionType) transactionv1.TransactionType {
	switch value {
	case domain.TransactionTypeIncome:
		return transactionv1.TransactionType_TRANSACTION_TYPE_INCOME
	case domain.TransactionTypeExpense:
		return transactionv1.TransactionType_TRANSACTION_TYPE_EXPENSE
	default:
		return transactionv1.TransactionType_TRANSACTION_TYPE_UNSPECIFIED
	}
}

func splitTypeFromProto(value transactionv1.SplitType) domain.SplitType {
	switch value {
	case transactionv1.SplitType_SPLIT_TYPE_FIXED:
		return domain.SplitTypeFixed
	case transactionv1.SplitType_SPLIT_TYPE_RATIO:
		return domain.SplitTypeRatio
	default:
		return ""
	}
}

func splitTypeToProto(value domain.SplitType) transactionv1.SplitType {
	switch value {
	case domain.SplitTypeFixed:
		return transactionv1.SplitType_SPLIT_TYPE_FIXED
	case domain.SplitTypeRatio:
		return transactionv1.SplitType_SPLIT_TYPE_RATIO
	default:
		return transactionv1.SplitType_SPLIT_TYPE_UNSPECIFIED
	}
}

func timeFromProto(value *timestamppb.Timestamp) time.Time {
	if value == nil || !value.IsValid() {
		return time.Time{}
	}
	return value.AsTime()
}

func timePtrFromProto(value *timestamppb.Timestamp) *time.Time {
	result := timeFromProto(value)
	if result.IsZero() {
		return nil
	}
	return &result
}

func deref(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func mapError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, apperrors.ErrValidation),
		errors.Is(err, apperrors.ErrInvalidSplit),
		errors.Is(err, apperrors.ErrInvalidPayment),
		errors.Is(err, apperrors.ErrCurrencyMismatch):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, apperrors.ErrNotFound),
		errors.Is(err, apperrors.ErrCategoryNotFound),
		errors.Is(err, apperrors.ErrTransactionNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, apperrors.ErrForbidden):
		return status.Error(codes.PermissionDenied, err.Error())
	case errors.Is(err, apperrors.ErrConflict):
		return status.Error(codes.AlreadyExists, err.Error())
	case errors.Is(err, apperrors.ErrUpstreamUnavailable):
		return status.Error(codes.Unavailable, err.Error())
	case errors.Is(err, context.DeadlineExceeded):
		return status.Error(codes.DeadlineExceeded, "deadline exceeded")
	case errors.Is(err, context.Canceled):
		return status.Error(codes.Canceled, "request canceled")
	default:
		return status.Error(codes.Internal, "internal server error")
	}
}
