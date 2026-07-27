package httptransport

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	transactionv1 "github.com/hiendangba/MoneyTrackingBE/Backend/transaction-service/gen/transaction/v1"

	"google.golang.org/protobuf/types/known/timestamppb"
)

func (g *Gateway) CreateCategory(w http.ResponseWriter, r *http.Request, claims AuthClaims) {
	var req CreateCategoryRequest
	if err := g.decodeJSON(w, r, &req); err != nil {
		writeError(w, err, g.logger)
		return
	}
	ctx, cancel := requestContext(r, g.cfg.RequestTimeout)
	defer cancel()
	response, err := g.transactionClient.CreateCategory(
		g.upstreamContext(ctx, r, &claims),
		&transactionv1.CreateCategoryRequest{
			Code: req.Code, Name: req.Name, Description: req.Description,
		},
	)
	if err != nil {
		writeError(w, err, g.logger)
		return
	}
	writeJSON(w, http.StatusCreated, toCategory(response))
}

func (g *Gateway) ListCategories(w http.ResponseWriter, r *http.Request, claims AuthClaims) {
	ctx, cancel := requestContext(r, g.cfg.RequestTimeout)
	defer cancel()
	includeInactive, _ := strconv.ParseBool(strings.TrimSpace(r.URL.Query().Get("include_inactive")))
	response, err := g.transactionClient.ListCategories(
		g.upstreamContext(ctx, r, &claims),
		&transactionv1.ListCategoriesRequest{IncludeInactive: includeInactive},
	)
	if err != nil {
		writeError(w, err, g.logger)
		return
	}
	categories := make([]Category, 0, len(response.GetCategories()))
	for _, category := range response.GetCategories() {
		categories = append(categories, toCategory(category))
	}
	writeJSON(w, http.StatusOK, categories)
}

func (g *Gateway) CreatePersonalTransaction(w http.ResponseWriter, r *http.Request, claims AuthClaims) {
	var req CreatePersonalTransactionRequest
	if err := g.decodeJSON(w, r, &req); err != nil {
		writeError(w, err, g.logger)
		return
	}
	ctx, cancel := requestContext(r, g.cfg.RequestTimeout)
	defer cancel()
	response, err := g.transactionClient.CreatePersonalTransaction(
		g.upstreamContext(ctx, r, &claims),
		&transactionv1.CreatePersonalTransactionRequest{
			UserId: claims.UserID, CategoryId: req.CategoryID, Type: transactionTypeFromString(req.Type),
			Title: req.Title, Amount: req.Amount, Currency: req.Currency,
			TransactionDate: optionalTimestamp(req.TransactionDate), Note: req.Note,
		},
	)
	if err != nil {
		writeError(w, err, g.logger)
		return
	}
	writeJSON(w, http.StatusCreated, toPersonalTransaction(response))
}

func (g *Gateway) GetPersonalTransaction(w http.ResponseWriter, r *http.Request, claims AuthClaims) {
	ctx, cancel := requestContext(r, g.cfg.RequestTimeout)
	defer cancel()
	response, err := g.transactionClient.GetPersonalTransaction(
		g.upstreamContext(ctx, r, &claims),
		&transactionv1.GetPersonalTransactionRequest{Id: r.PathValue("id"), UserId: claims.UserID},
	)
	if err != nil {
		writeError(w, err, g.logger)
		return
	}
	writeJSON(w, http.StatusOK, toPersonalTransaction(response))
}

func (g *Gateway) ListPersonalTransactions(w http.ResponseWriter, r *http.Request, claims AuthClaims) {
	ctx, cancel := requestContext(r, g.cfg.RequestTimeout)
	defer cancel()
	query := r.URL.Query()
	response, err := g.transactionClient.ListPersonalTransactions(
		g.upstreamContext(ctx, r, &claims),
		&transactionv1.ListPersonalTransactionsRequest{
			UserId: claims.UserID, From: parseTimestamp(query.Get("from")), To: parseTimestamp(query.Get("to")),
			Type: transactionTypeFromString(query.Get("type")), CategoryId: query.Get("category_id"),
			Limit: int32(parseInt(query.Get("limit"), 50)), Offset: int32(parseInt(query.Get("offset"), 0)),
		},
	)
	if err != nil {
		writeError(w, err, g.logger)
		return
	}
	items := make([]PersonalTransaction, 0, len(response.GetTransactions()))
	for _, transaction := range response.GetTransactions() {
		items = append(items, toPersonalTransaction(transaction))
	}
	writeJSON(w, http.StatusOK, PaginatedPersonalTransactions{Items: items, Total: response.GetTotal()})
}

func (g *Gateway) DeletePersonalTransaction(w http.ResponseWriter, r *http.Request, claims AuthClaims) {
	ctx, cancel := requestContext(r, g.cfg.RequestTimeout)
	defer cancel()
	response, err := g.transactionClient.DeletePersonalTransaction(
		g.upstreamContext(ctx, r, &claims),
		&transactionv1.DeletePersonalTransactionRequest{Id: r.PathValue("id"), UserId: claims.UserID},
	)
	if err != nil {
		writeError(w, err, g.logger)
		return
	}
	writeJSON(w, http.StatusOK, MessageResponse{Message: response.GetMessage()})
}

func (g *Gateway) CreateGroupTransaction(w http.ResponseWriter, r *http.Request, claims AuthClaims) {
	var req CreateGroupTransactionRequest
	if err := g.decodeJSON(w, r, &req); err != nil {
		writeError(w, err, g.logger)
		return
	}
	payments := make([]*transactionv1.PaymentInput, 0, len(req.Payments))
	for _, payment := range req.Payments {
		payments = append(payments, &transactionv1.PaymentInput{
			UserId: payment.UserID, Amount: payment.Amount, Note: payment.Note,
		})
	}
	splits := make([]*transactionv1.SplitInput, 0, len(req.Splits))
	for _, split := range req.Splits {
		splits = append(splits, &transactionv1.SplitInput{
			UserId: split.UserID, SplitType: splitTypeFromString(split.SplitType),
			SplitValue: split.SplitValue,
		})
	}
	ctx, cancel := requestContext(r, g.cfg.RequestTimeout)
	defer cancel()
	response, err := g.transactionClient.CreateGroupTransaction(
		g.upstreamContext(ctx, r, &claims),
		&transactionv1.CreateGroupTransactionRequest{
			GroupId: r.PathValue("id"), CategoryId: req.CategoryID,
			Type: transactionTypeFromString(req.Type), Title: req.Title, Amount: req.Amount,
			Currency: req.Currency, TransactionDate: optionalTimestamp(req.TransactionDate),
			Note: req.Note, CreatedBy: claims.UserID, Payments: payments, Splits: splits,
		},
	)
	if err != nil {
		writeError(w, err, g.logger)
		return
	}
	writeJSON(w, http.StatusCreated, toGroupTransaction(response))
}

func (g *Gateway) GetGroupTransaction(w http.ResponseWriter, r *http.Request, claims AuthClaims) {
	ctx, cancel := requestContext(r, g.cfg.RequestTimeout)
	defer cancel()
	response, err := g.transactionClient.GetGroupTransaction(
		g.upstreamContext(ctx, r, &claims),
		&transactionv1.GetGroupTransactionRequest{
			Id: r.PathValue("transaction_id"), UserId: claims.UserID,
		},
	)
	if err != nil {
		writeError(w, err, g.logger)
		return
	}
	writeJSON(w, http.StatusOK, toGroupTransaction(response))
}

func (g *Gateway) ListGroupTransactions(w http.ResponseWriter, r *http.Request, claims AuthClaims) {
	ctx, cancel := requestContext(r, g.cfg.RequestTimeout)
	defer cancel()
	query := r.URL.Query()
	response, err := g.transactionClient.ListGroupTransactions(
		g.upstreamContext(ctx, r, &claims),
		&transactionv1.ListGroupTransactionsRequest{
			GroupId: r.PathValue("id"), UserId: claims.UserID,
			From: parseTimestamp(query.Get("from")), To: parseTimestamp(query.Get("to")),
			Type: transactionTypeFromString(query.Get("type")), CategoryId: query.Get("category_id"),
			Limit: int32(parseInt(query.Get("limit"), 50)), Offset: int32(parseInt(query.Get("offset"), 0)),
		},
	)
	if err != nil {
		writeError(w, err, g.logger)
		return
	}
	items := make([]GroupTransaction, 0, len(response.GetTransactions()))
	for _, transaction := range response.GetTransactions() {
		items = append(items, toGroupTransaction(transaction))
	}
	writeJSON(w, http.StatusOK, PaginatedGroupTransactions{Items: items, Total: response.GetTotal()})
}

func (g *Gateway) DeleteGroupTransaction(w http.ResponseWriter, r *http.Request, claims AuthClaims) {
	ctx, cancel := requestContext(r, g.cfg.RequestTimeout)
	defer cancel()
	response, err := g.transactionClient.DeleteGroupTransaction(
		g.upstreamContext(ctx, r, &claims),
		&transactionv1.DeleteGroupTransactionRequest{
			Id: r.PathValue("transaction_id"), UserId: claims.UserID,
		},
	)
	if err != nil {
		writeError(w, err, g.logger)
		return
	}
	writeJSON(w, http.StatusOK, MessageResponse{Message: response.GetMessage()})
}

func (g *Gateway) CreateSettlement(w http.ResponseWriter, r *http.Request, claims AuthClaims) {
	var req CreateSettlementRequest
	if err := g.decodeJSON(w, r, &req); err != nil {
		writeError(w, err, g.logger)
		return
	}
	ctx, cancel := requestContext(r, g.cfg.RequestTimeout)
	defer cancel()
	response, err := g.transactionClient.CreateSettlement(
		g.upstreamContext(ctx, r, &claims),
		&transactionv1.CreateSettlementRequest{
			GroupId: r.PathValue("id"), FromUserId: req.FromUserID, ToUserId: req.ToUserID,
			Amount: req.Amount, Currency: req.Currency, SettledAt: optionalTimestamp(req.SettledAt),
			Note: req.Note, CreatedBy: claims.UserID,
		},
	)
	if err != nil {
		writeError(w, err, g.logger)
		return
	}
	writeJSON(w, http.StatusCreated, toSettlement(response))
}

func (g *Gateway) ListSettlements(w http.ResponseWriter, r *http.Request, claims AuthClaims) {
	ctx, cancel := requestContext(r, g.cfg.RequestTimeout)
	defer cancel()
	query := r.URL.Query()
	response, err := g.transactionClient.ListSettlements(
		g.upstreamContext(ctx, r, &claims),
		&transactionv1.ListSettlementsRequest{
			GroupId: r.PathValue("id"), UserId: claims.UserID,
			Limit: int32(parseInt(query.Get("limit"), 50)), Offset: int32(parseInt(query.Get("offset"), 0)),
		},
	)
	if err != nil {
		writeError(w, err, g.logger)
		return
	}
	items := make([]Settlement, 0, len(response.GetSettlements()))
	for _, settlement := range response.GetSettlements() {
		items = append(items, toSettlement(settlement))
	}
	writeJSON(w, http.StatusOK, PaginatedSettlements{Items: items, Total: response.GetTotal()})
}

func (g *Gateway) GetGroupBalances(w http.ResponseWriter, r *http.Request, claims AuthClaims) {
	ctx, cancel := requestContext(r, g.cfg.RequestTimeout)
	defer cancel()
	response, err := g.transactionClient.GetGroupBalances(
		g.upstreamContext(ctx, r, &claims),
		&transactionv1.GetGroupBalancesRequest{
			GroupId: r.PathValue("id"), UserId: claims.UserID, Currency: r.URL.Query().Get("currency"),
		},
	)
	if err != nil {
		writeError(w, err, g.logger)
		return
	}
	balances := make([]MemberBalance, 0, len(response.GetBalances()))
	for _, balance := range response.GetBalances() {
		balances = append(balances, MemberBalance{
			User: toUserSnapshot(balance.GetUser()), Paid: balance.GetPaid(), Share: balance.GetShare(),
			SettlementsSent:     balance.GetSettlementsSent(),
			SettlementsReceived: balance.GetSettlementsReceived(), NetBalance: balance.GetNetBalance(),
		})
	}
	suggestions := make([]SettlementSuggestion, 0, len(response.GetSuggestions()))
	for _, suggestion := range response.GetSuggestions() {
		suggestions = append(suggestions, SettlementSuggestion{
			FromUser: toUserSnapshot(suggestion.GetFromUser()),
			ToUser:   toUserSnapshot(suggestion.GetToUser()),
			Amount:   suggestion.GetAmount(), Currency: suggestion.GetCurrency(),
		})
	}
	writeJSON(w, http.StatusOK, GroupBalancesResponse{Balances: balances, Suggestions: suggestions})
}

func toCategory(value *transactionv1.Category) Category {
	if value == nil {
		return Category{}
	}
	return Category{
		ID: value.GetId(), Code: value.GetCode(), Name: value.GetName(),
		Description: stringPtr(value.GetDescription()), IsActive: value.GetIsActive(),
		CreatedAt: protoTime(value.GetCreatedAt()), UpdatedAt: protoTime(value.GetUpdatedAt()),
	}
}

func toUserSnapshot(value *transactionv1.UserSnapshot) UserSnapshot {
	if value == nil {
		return UserSnapshot{}
	}
	return UserSnapshot{UserID: value.GetUserId(), Fullname: value.GetFullname(), Email: value.GetEmail()}
}

func toPersonalTransaction(value *transactionv1.PersonalTransaction) PersonalTransaction {
	if value == nil {
		return PersonalTransaction{}
	}
	return PersonalTransaction{
		ID: value.GetId(), User: toUserSnapshot(value.GetUser()), Category: categoryPtr(value.GetCategory()),
		Type: transactionTypeToString(value.GetType()), Title: value.GetTitle(), Amount: value.GetAmount(),
		Currency: value.GetCurrency(), TransactionDate: protoTime(value.GetTransactionDate()),
		Note: stringPtr(value.GetNote()), CreatedAt: protoTime(value.GetCreatedAt()),
		UpdatedAt: protoTime(value.GetUpdatedAt()),
	}
}

func toGroupTransaction(value *transactionv1.GroupTransaction) GroupTransaction {
	if value == nil {
		return GroupTransaction{}
	}
	transaction := GroupTransaction{
		ID: value.GetId(), GroupID: value.GetGroupId(), GroupName: value.GetGroupName(),
		Category: categoryPtr(value.GetCategory()), Type: transactionTypeToString(value.GetType()),
		Title: value.GetTitle(), Amount: value.GetAmount(), Currency: value.GetCurrency(),
		TransactionDate: protoTime(value.GetTransactionDate()), Note: stringPtr(value.GetNote()),
		CreatedBy: toUserSnapshot(value.GetCreatedBy()),
		Payments:  make([]GroupPayment, 0, len(value.GetPayments())),
		Splits:    make([]GroupSplit, 0, len(value.GetSplits())),
		CreatedAt: protoTime(value.GetCreatedAt()), UpdatedAt: protoTime(value.GetUpdatedAt()),
	}
	for _, payment := range value.GetPayments() {
		transaction.Payments = append(transaction.Payments, GroupPayment{
			ID: payment.GetId(), User: toUserSnapshot(payment.GetUser()), Amount: payment.GetAmount(),
			Note: stringPtr(payment.GetNote()), CreatedAt: protoTime(payment.GetCreatedAt()),
		})
	}
	for _, split := range value.GetSplits() {
		transaction.Splits = append(transaction.Splits, GroupSplit{
			ID: split.GetId(), User: toUserSnapshot(split.GetUser()),
			SplitType: splitTypeToString(split.GetSplitType()), SplitValue: split.GetSplitValue(),
			ShareAmount: split.GetShareAmount(), CreatedAt: protoTime(split.GetCreatedAt()),
		})
	}
	return transaction
}

func toSettlement(value *transactionv1.Settlement) Settlement {
	if value == nil {
		return Settlement{}
	}
	return Settlement{
		ID: value.GetId(), GroupID: value.GetGroupId(), GroupName: value.GetGroupName(),
		FromUser: toUserSnapshot(value.GetFromUser()), ToUser: toUserSnapshot(value.GetToUser()),
		Amount: value.GetAmount(), Currency: value.GetCurrency(), SettledAt: protoTime(value.GetSettledAt()),
		Note: stringPtr(value.GetNote()), CreatedBy: toUserSnapshot(value.GetCreatedBy()),
		CreatedAt: protoTime(value.GetCreatedAt()),
	}
}

func categoryPtr(value *transactionv1.Category) *Category {
	if value == nil {
		return nil
	}
	category := toCategory(value)
	return &category
}

func transactionTypeFromString(value string) transactionv1.TransactionType {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "income":
		return transactionv1.TransactionType_TRANSACTION_TYPE_INCOME
	case "expense":
		return transactionv1.TransactionType_TRANSACTION_TYPE_EXPENSE
	default:
		return transactionv1.TransactionType_TRANSACTION_TYPE_UNSPECIFIED
	}
}

func transactionTypeToString(value transactionv1.TransactionType) string {
	switch value {
	case transactionv1.TransactionType_TRANSACTION_TYPE_INCOME:
		return "income"
	case transactionv1.TransactionType_TRANSACTION_TYPE_EXPENSE:
		return "expense"
	default:
		return "unspecified"
	}
}

func splitTypeFromString(value string) transactionv1.SplitType {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "fixed":
		return transactionv1.SplitType_SPLIT_TYPE_FIXED
	case "ratio":
		return transactionv1.SplitType_SPLIT_TYPE_RATIO
	default:
		return transactionv1.SplitType_SPLIT_TYPE_UNSPECIFIED
	}
}

func splitTypeToString(value transactionv1.SplitType) string {
	switch value {
	case transactionv1.SplitType_SPLIT_TYPE_FIXED:
		return "fixed"
	case transactionv1.SplitType_SPLIT_TYPE_RATIO:
		return "ratio"
	default:
		return "unspecified"
	}
}

func parseTimestamp(value string) *timestamppb.Timestamp {
	parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(value))
	if err != nil {
		return nil
	}
	return timestamppb.New(parsed)
}

func optionalTimestamp(value time.Time) *timestamppb.Timestamp {
	if value.IsZero() {
		return nil
	}
	return timestamppb.New(value)
}

func protoTime(value *timestamppb.Timestamp) time.Time {
	if value == nil || !value.IsValid() {
		return time.Time{}
	}
	return value.AsTime()
}

func parseInt(value string, fallback int) int {
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return fallback
	}
	return parsed
}

func stringPtr(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return &value
}
