package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"transaction-service/internal/domain"
	apperrors "transaction-service/internal/errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type dbExecutor interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
}

type PostgresRepository struct {
	db *pgxpool.Pool
}

func NewPostgresRepository(db *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{db: db}
}

func (r *PostgresRepository) CreateCategory(ctx context.Context, category domain.Category) (*domain.Category, error) {
	const query = `
		INSERT INTO expense_categories (id, code, name, description, is_active, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, code, name, description, is_active, created_at, updated_at, deleted_at`
	created, err := scanCategory(r.db.QueryRow(ctx, query,
		category.ID, category.Code, category.Name, category.Description,
		category.IsActive, category.CreatedAt, category.UpdatedAt,
	))
	if err != nil {
		if isUniqueViolation(err) {
			return nil, apperrors.ErrConflict
		}
		return nil, fmt.Errorf("create category: %w", err)
	}
	return created, nil
}

func (r *PostgresRepository) GetCategory(ctx context.Context, id string) (*domain.Category, error) {
	const query = `
		SELECT id, code, name, description, is_active, created_at, updated_at, deleted_at
		FROM expense_categories
		WHERE id = $1 AND deleted_at IS NULL`
	category, err := scanCategory(r.db.QueryRow(ctx, query, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.ErrCategoryNotFound
		}
		return nil, fmt.Errorf("get category: %w", err)
	}
	return category, nil
}

func (r *PostgresRepository) ListCategories(ctx context.Context, includeInactive bool) ([]domain.Category, error) {
	query := `
		SELECT id, code, name, description, is_active, created_at, updated_at, deleted_at
		FROM expense_categories WHERE deleted_at IS NULL`
	if !includeInactive {
		query += " AND is_active = TRUE"
	}
	query += " ORDER BY name ASC"
	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list categories: %w", err)
	}
	defer rows.Close()
	result := make([]domain.Category, 0)
	for rows.Next() {
		category, scanErr := scanCategory(rows)
		if scanErr != nil {
			return nil, fmt.Errorf("scan category: %w", scanErr)
		}
		result = append(result, *category)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate categories: %w", err)
	}
	return result, nil
}

func (r *PostgresRepository) CreatePersonalTransaction(ctx context.Context, transaction domain.PersonalTransaction) (*domain.PersonalTransaction, error) {
	const query = `
		INSERT INTO personal_transactions (
			id, user_id, user_fullname, user_email, category_id, category_code, category_name,
			type, title, amount, currency, transaction_date, note, created_at, updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)
		RETURNING id, user_id, user_fullname, user_email, category_id, category_code, category_name,
		          type, title, amount::text, currency, transaction_date, note, created_at, updated_at, deleted_at`
	created, err := scanPersonal(r.db.QueryRow(ctx, query,
		transaction.ID, transaction.User.UserID, transaction.User.Fullname, transaction.User.Email,
		transaction.CategoryID, transaction.CategoryCode, transaction.CategoryName, transaction.Type,
		transaction.Title, domain.FormatMoney(transaction.AmountMinor), transaction.Currency,
		transaction.TransactionDate, transaction.Note, transaction.CreatedAt, transaction.UpdatedAt,
	))
	if err != nil {
		return nil, fmt.Errorf("create personal transaction: %w", err)
	}
	return created, nil
}

func (r *PostgresRepository) GetPersonalTransaction(ctx context.Context, id, userID string) (*domain.PersonalTransaction, error) {
	const query = `
		SELECT id, user_id, user_fullname, user_email, category_id, category_code, category_name,
		       type, title, amount::text, currency, transaction_date, note, created_at, updated_at, deleted_at
		FROM personal_transactions
		WHERE id = $1 AND user_id = $2 AND deleted_at IS NULL`
	transaction, err := scanPersonal(r.db.QueryRow(ctx, query, id, userID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.ErrTransactionNotFound
		}
		return nil, fmt.Errorf("get personal transaction: %w", err)
	}
	return transaction, nil
}

func (r *PostgresRepository) ListPersonalTransactions(
	ctx context.Context,
	userID string,
	filter domain.ListFilter,
) ([]domain.PersonalTransaction, int64, error) {
	query := `
		SELECT id, user_id, user_fullname, user_email, category_id, category_code, category_name,
		       type, title, amount::text, currency, transaction_date, note, created_at, updated_at, deleted_at,
		       COUNT(*) OVER()
		FROM personal_transactions
		WHERE user_id = $1 AND deleted_at IS NULL`
	args := []any{userID}
	query = appendFilters(query, &args, filter)
	query += fmt.Sprintf(" ORDER BY transaction_date DESC, id DESC LIMIT $%d OFFSET $%d", len(args)+1, len(args)+2)
	args = append(args, filter.Limit, filter.Offset)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list personal transactions: %w", err)
	}
	defer rows.Close()
	result := make([]domain.PersonalTransaction, 0)
	var total int64
	for rows.Next() {
		transaction, count, scanErr := scanPersonalWithCount(rows)
		if scanErr != nil {
			return nil, 0, fmt.Errorf("scan personal transaction: %w", scanErr)
		}
		total = count
		result = append(result, *transaction)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate personal transactions: %w", err)
	}
	return result, total, nil
}

func (r *PostgresRepository) DeletePersonalTransaction(ctx context.Context, id, userID string) error {
	const query = `
		UPDATE personal_transactions SET deleted_at = NOW(), updated_at = NOW()
		WHERE id = $1 AND user_id = $2 AND deleted_at IS NULL`
	tag, err := r.db.Exec(ctx, query, id, userID)
	if err != nil {
		return fmt.Errorf("delete personal transaction: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return apperrors.ErrTransactionNotFound
	}
	return nil
}

func (r *PostgresRepository) CreateGroupTransaction(ctx context.Context, transaction domain.GroupTransaction) (*domain.GroupTransaction, error) {
	tx, err := r.db.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
	if err != nil {
		return nil, fmt.Errorf("begin group transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	const insertTransaction = `
		INSERT INTO group_transactions (
			id, group_id, group_name, category_id, category_code, category_name, type, title,
			amount, currency, transaction_date, note, created_by, created_by_fullname,
			created_by_email, created_at, updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17)`
	if _, err := tx.Exec(ctx, insertTransaction,
		transaction.ID, transaction.GroupID, transaction.GroupName, transaction.CategoryID,
		transaction.CategoryCode, transaction.CategoryName, transaction.Type, transaction.Title,
		domain.FormatMoney(transaction.AmountMinor), transaction.Currency, transaction.TransactionDate,
		transaction.Note, transaction.CreatedBy.UserID, transaction.CreatedBy.Fullname,
		transaction.CreatedBy.Email, transaction.CreatedAt, transaction.UpdatedAt,
	); err != nil {
		return nil, fmt.Errorf("insert group transaction: %w", err)
	}
	const insertPayment = `
		INSERT INTO group_transaction_payments (
			id, transaction_id, user_id, user_fullname, user_email, amount, note, created_at, updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`
	for _, payment := range transaction.Payments {
		if _, err := tx.Exec(ctx, insertPayment,
			payment.ID, transaction.ID, payment.User.UserID, payment.User.Fullname, payment.User.Email,
			domain.FormatMoney(payment.AmountMinor), payment.Note, payment.CreatedAt, payment.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("insert group payment: %w", err)
		}
	}
	const insertSplit = `
		INSERT INTO group_transaction_splits (
			id, transaction_id, user_id, user_fullname, user_email, split_type,
			split_value, share_amount, created_at, updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`
	for _, split := range transaction.Splits {
		value := domain.FormatRatio(split.SplitValue)
		if split.SplitType == domain.SplitTypeFixed {
			value = domain.FormatMoney(split.SplitValue)
		}
		if _, err := tx.Exec(ctx, insertSplit,
			split.ID, transaction.ID, split.User.UserID, split.User.Fullname, split.User.Email,
			split.SplitType, value, domain.FormatMoney(split.ShareMinor), split.CreatedAt, split.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("insert group split: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit group transaction: %w", err)
	}
	return r.GetGroupTransaction(ctx, transaction.ID)
}

func (r *PostgresRepository) GetGroupTransaction(ctx context.Context, id string) (*domain.GroupTransaction, error) {
	transaction, err := scanGroupTransaction(r.db.QueryRow(ctx, groupTransactionSelect+" WHERE gt.id = $1 AND gt.deleted_at IS NULL", id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, apperrors.ErrTransactionNotFound
		}
		return nil, fmt.Errorf("get group transaction: %w", err)
	}
	if err := r.loadGroupChildren(ctx, r.db, transaction); err != nil {
		return nil, err
	}
	return transaction, nil
}

func (r *PostgresRepository) ListGroupTransactions(
	ctx context.Context,
	groupID string,
	filter domain.ListFilter,
) ([]domain.GroupTransaction, int64, error) {
	query := groupTransactionSelect + " WHERE gt.group_id = $1 AND gt.deleted_at IS NULL"
	args := []any{groupID}
	query = appendFiltersWithAlias(query, &args, filter, "gt")
	query += fmt.Sprintf(" ORDER BY gt.transaction_date DESC, gt.id DESC LIMIT $%d OFFSET $%d", len(args)+1, len(args)+2)
	args = append(args, filter.Limit, filter.Offset)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list group transactions: %w", err)
	}
	defer rows.Close()
	result := make([]domain.GroupTransaction, 0)
	for rows.Next() {
		transaction, scanErr := scanGroupTransaction(rows)
		if scanErr != nil {
			return nil, 0, fmt.Errorf("scan group transaction: %w", scanErr)
		}
		result = append(result, *transaction)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate group transactions: %w", err)
	}
	var total int64
	countQuery := "SELECT COUNT(*) FROM group_transactions gt WHERE gt.group_id = $1 AND gt.deleted_at IS NULL"
	countArgs := []any{groupID}
	countQuery = appendFiltersWithAlias(countQuery, &countArgs, filter, "gt")
	if err := r.db.QueryRow(ctx, countQuery, countArgs...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count group transactions: %w", err)
	}
	for i := range result {
		if err := r.loadGroupChildren(ctx, r.db, &result[i]); err != nil {
			return nil, 0, err
		}
	}
	return result, total, nil
}

func (r *PostgresRepository) DeleteGroupTransaction(ctx context.Context, id string) error {
	const query = `
		UPDATE group_transactions SET deleted_at = NOW(), updated_at = NOW()
		WHERE id = $1 AND deleted_at IS NULL`
	tag, err := r.db.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete group transaction: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return apperrors.ErrTransactionNotFound
	}
	return nil
}

func (r *PostgresRepository) CreateSettlement(ctx context.Context, settlement domain.Settlement) (*domain.Settlement, error) {
	const query = `
		INSERT INTO settlements (
			id, group_id, group_name, from_user_id, from_user_fullname, from_user_email,
			to_user_id, to_user_fullname, to_user_email, amount, currency, settled_at,
			note, created_by, created_by_fullname, created_by_email, created_at, updated_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18)
		RETURNING id, group_id, group_name, from_user_id, from_user_fullname, from_user_email,
		          to_user_id, to_user_fullname, to_user_email, amount::text, currency, settled_at,
		          note, created_by, created_by_fullname, created_by_email, created_at, updated_at, deleted_at`
	created, err := scanSettlement(r.db.QueryRow(ctx, query,
		settlement.ID, settlement.GroupID, settlement.GroupName,
		settlement.FromUser.UserID, settlement.FromUser.Fullname, settlement.FromUser.Email,
		settlement.ToUser.UserID, settlement.ToUser.Fullname, settlement.ToUser.Email,
		domain.FormatMoney(settlement.AmountMinor), settlement.Currency, settlement.SettledAt,
		settlement.Note, settlement.CreatedBy.UserID, settlement.CreatedBy.Fullname,
		settlement.CreatedBy.Email, settlement.CreatedAt, settlement.UpdatedAt,
	))
	if err != nil {
		return nil, fmt.Errorf("create settlement: %w", err)
	}
	return created, nil
}

func (r *PostgresRepository) ListSettlements(ctx context.Context, groupID string, limit, offset int) ([]domain.Settlement, int64, error) {
	const query = `
		SELECT id, group_id, group_name, from_user_id, from_user_fullname, from_user_email,
		       to_user_id, to_user_fullname, to_user_email, amount::text, currency, settled_at,
		       note, created_by, created_by_fullname, created_by_email, created_at, updated_at, deleted_at,
		       COUNT(*) OVER()
		FROM settlements
		WHERE group_id = $1 AND deleted_at IS NULL
		ORDER BY settled_at DESC, id DESC LIMIT $2 OFFSET $3`
	rows, err := r.db.Query(ctx, query, groupID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list settlements: %w", err)
	}
	defer rows.Close()
	result := make([]domain.Settlement, 0)
	var total int64
	for rows.Next() {
		settlement, count, scanErr := scanSettlementWithCount(rows)
		if scanErr != nil {
			return nil, 0, fmt.Errorf("scan settlement: %w", scanErr)
		}
		total = count
		result = append(result, *settlement)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate settlements: %w", err)
	}
	return result, total, nil
}

func (r *PostgresRepository) GetGroupLedger(ctx context.Context, groupID, currency string) ([]domain.GroupTransaction, []domain.Settlement, error) {
	const transactionQuery = `
		SELECT id FROM group_transactions
		WHERE group_id = $1 AND currency = $2 AND deleted_at IS NULL
		ORDER BY transaction_date ASC, id ASC`
	rows, err := r.db.Query(ctx, transactionQuery, groupID, currency)
	if err != nil {
		return nil, nil, fmt.Errorf("list ledger transaction ids: %w", err)
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, nil, fmt.Errorf("scan ledger transaction id: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, nil, fmt.Errorf("iterate ledger transaction ids: %w", err)
	}
	rows.Close()
	transactions := make([]domain.GroupTransaction, 0, len(ids))
	for _, id := range ids {
		transaction, getErr := r.GetGroupTransaction(ctx, id)
		if getErr != nil {
			return nil, nil, getErr
		}
		transactions = append(transactions, *transaction)
	}
	const settlementQuery = `
		SELECT id, group_id, group_name, from_user_id, from_user_fullname, from_user_email,
		       to_user_id, to_user_fullname, to_user_email, amount::text, currency, settled_at,
		       note, created_by, created_by_fullname, created_by_email, created_at, updated_at, deleted_at
		FROM settlements
		WHERE group_id = $1 AND currency = $2 AND deleted_at IS NULL
		ORDER BY settled_at ASC, id ASC`
	settlementRows, err := r.db.Query(ctx, settlementQuery, groupID, currency)
	if err != nil {
		return nil, nil, fmt.Errorf("list ledger settlements: %w", err)
	}
	defer settlementRows.Close()
	settlements := make([]domain.Settlement, 0)
	for settlementRows.Next() {
		settlement, scanErr := scanSettlement(settlementRows)
		if scanErr != nil {
			return nil, nil, fmt.Errorf("scan ledger settlement: %w", scanErr)
		}
		settlements = append(settlements, *settlement)
	}
	if err := settlementRows.Err(); err != nil {
		return nil, nil, fmt.Errorf("iterate ledger settlements: %w", err)
	}
	return transactions, settlements, nil
}

func (r *PostgresRepository) loadGroupChildren(ctx context.Context, db dbExecutor, transaction *domain.GroupTransaction) error {
	const paymentQuery = `
		SELECT id, transaction_id, user_id, user_fullname, user_email, amount::text, note, created_at, updated_at
		FROM group_transaction_payments WHERE transaction_id = $1 ORDER BY created_at ASC, id ASC`
	rows, err := db.Query(ctx, paymentQuery, transaction.ID)
	if err != nil {
		return fmt.Errorf("list group payments: %w", err)
	}
	for rows.Next() {
		payment, scanErr := scanPayment(rows)
		if scanErr != nil {
			rows.Close()
			return fmt.Errorf("scan group payment: %w", scanErr)
		}
		transaction.Payments = append(transaction.Payments, *payment)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return fmt.Errorf("iterate group payments: %w", err)
	}
	rows.Close()

	const splitQuery = `
		SELECT id, transaction_id, user_id, user_fullname, user_email, split_type,
		       split_value::text, share_amount::text, created_at, updated_at
		FROM group_transaction_splits WHERE transaction_id = $1 ORDER BY created_at ASC, id ASC`
	rows, err = db.Query(ctx, splitQuery, transaction.ID)
	if err != nil {
		return fmt.Errorf("list group splits: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		split, scanErr := scanSplit(rows)
		if scanErr != nil {
			return fmt.Errorf("scan group split: %w", scanErr)
		}
		transaction.Splits = append(transaction.Splits, *split)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate group splits: %w", err)
	}
	return nil
}

const groupTransactionSelect = `
	SELECT gt.id, gt.group_id, gt.group_name, gt.category_id, gt.category_code, gt.category_name,
	       gt.type, gt.title, gt.amount::text, gt.currency, gt.transaction_date, gt.note,
	       gt.created_by, gt.created_by_fullname, gt.created_by_email,
	       gt.created_at, gt.updated_at, gt.deleted_at
	FROM group_transactions gt`

func appendFilters(query string, args *[]any, filter domain.ListFilter) string {
	return appendFiltersWithAlias(query, args, filter, "")
}

func appendFiltersWithAlias(query string, args *[]any, filter domain.ListFilter, alias string) string {
	prefix := ""
	if alias != "" {
		prefix = alias + "."
	}
	if filter.From != nil {
		*args = append(*args, *filter.From)
		query += fmt.Sprintf(" AND %stransaction_date >= $%d", prefix, len(*args))
	}
	if filter.To != nil {
		*args = append(*args, *filter.To)
		query += fmt.Sprintf(" AND %stransaction_date <= $%d", prefix, len(*args))
	}
	if filter.Type != "" {
		*args = append(*args, filter.Type)
		query += fmt.Sprintf(" AND %stype = $%d", prefix, len(*args))
	}
	if filter.CategoryID != "" {
		*args = append(*args, filter.CategoryID)
		query += fmt.Sprintf(" AND %scategory_id = $%d", prefix, len(*args))
	}
	return query
}

func scanCategory(row pgx.Row) (*domain.Category, error) {
	category := &domain.Category{}
	if err := row.Scan(
		&category.ID, &category.Code, &category.Name, &category.Description, &category.IsActive,
		&category.CreatedAt, &category.UpdatedAt, &category.DeletedAt,
	); err != nil {
		return nil, err
	}
	return category, nil
}

func scanPersonal(row pgx.Row) (*domain.PersonalTransaction, error) {
	transaction := &domain.PersonalTransaction{}
	var amount string
	if err := row.Scan(
		&transaction.ID, &transaction.User.UserID, &transaction.User.Fullname, &transaction.User.Email,
		&transaction.CategoryID, &transaction.CategoryCode, &transaction.CategoryName,
		&transaction.Type, &transaction.Title, &amount, &transaction.Currency,
		&transaction.TransactionDate, &transaction.Note, &transaction.CreatedAt,
		&transaction.UpdatedAt, &transaction.DeletedAt,
	); err != nil {
		return nil, err
	}
	value, err := domain.ParseMoney(amount)
	if err != nil {
		return nil, fmt.Errorf("parse personal amount: %w", err)
	}
	transaction.AmountMinor = value
	return transaction, nil
}

func scanPersonalWithCount(row pgx.Row) (*domain.PersonalTransaction, int64, error) {
	transaction := &domain.PersonalTransaction{}
	var amount string
	var total int64
	if err := row.Scan(
		&transaction.ID, &transaction.User.UserID, &transaction.User.Fullname, &transaction.User.Email,
		&transaction.CategoryID, &transaction.CategoryCode, &transaction.CategoryName,
		&transaction.Type, &transaction.Title, &amount, &transaction.Currency,
		&transaction.TransactionDate, &transaction.Note, &transaction.CreatedAt,
		&transaction.UpdatedAt, &transaction.DeletedAt, &total,
	); err != nil {
		return nil, 0, err
	}
	value, err := domain.ParseMoney(amount)
	if err != nil {
		return nil, 0, fmt.Errorf("parse personal amount: %w", err)
	}
	transaction.AmountMinor = value
	return transaction, total, nil
}

func scanGroupTransaction(row pgx.Row) (*domain.GroupTransaction, error) {
	transaction := &domain.GroupTransaction{}
	var amount string
	if err := row.Scan(
		&transaction.ID, &transaction.GroupID, &transaction.GroupName,
		&transaction.CategoryID, &transaction.CategoryCode, &transaction.CategoryName,
		&transaction.Type, &transaction.Title, &amount, &transaction.Currency,
		&transaction.TransactionDate, &transaction.Note, &transaction.CreatedBy.UserID,
		&transaction.CreatedBy.Fullname, &transaction.CreatedBy.Email,
		&transaction.CreatedAt, &transaction.UpdatedAt, &transaction.DeletedAt,
	); err != nil {
		return nil, err
	}
	value, err := domain.ParseMoney(amount)
	if err != nil {
		return nil, fmt.Errorf("parse group amount: %w", err)
	}
	transaction.AmountMinor = value
	return transaction, nil
}

func scanPayment(row pgx.Row) (*domain.GroupPayment, error) {
	payment := &domain.GroupPayment{}
	var amount string
	if err := row.Scan(
		&payment.ID, &payment.TransactionID, &payment.User.UserID, &payment.User.Fullname,
		&payment.User.Email, &amount, &payment.Note, &payment.CreatedAt, &payment.UpdatedAt,
	); err != nil {
		return nil, err
	}
	value, err := domain.ParseMoney(amount)
	if err != nil {
		return nil, fmt.Errorf("parse payment amount: %w", err)
	}
	payment.AmountMinor = value
	return payment, nil
}

func scanSplit(row pgx.Row) (*domain.GroupSplit, error) {
	split := &domain.GroupSplit{}
	var value, share string
	if err := row.Scan(
		&split.ID, &split.TransactionID, &split.User.UserID, &split.User.Fullname,
		&split.User.Email, &split.SplitType, &value, &share, &split.CreatedAt, &split.UpdatedAt,
	); err != nil {
		return nil, err
	}
	var err error
	if split.SplitType == domain.SplitTypeFixed {
		split.SplitValue, err = domain.ParseMoney(value)
	} else {
		split.SplitValue, err = domain.ParseRatio(value)
	}
	if err != nil {
		return nil, fmt.Errorf("parse split value: %w", err)
	}
	split.ShareMinor, err = domain.ParseMoney(share)
	if err != nil {
		return nil, fmt.Errorf("parse share amount: %w", err)
	}
	return split, nil
}

func scanSettlement(row pgx.Row) (*domain.Settlement, error) {
	settlement := &domain.Settlement{}
	var amount string
	if err := row.Scan(
		&settlement.ID, &settlement.GroupID, &settlement.GroupName,
		&settlement.FromUser.UserID, &settlement.FromUser.Fullname, &settlement.FromUser.Email,
		&settlement.ToUser.UserID, &settlement.ToUser.Fullname, &settlement.ToUser.Email,
		&amount, &settlement.Currency, &settlement.SettledAt, &settlement.Note,
		&settlement.CreatedBy.UserID, &settlement.CreatedBy.Fullname, &settlement.CreatedBy.Email,
		&settlement.CreatedAt, &settlement.UpdatedAt, &settlement.DeletedAt,
	); err != nil {
		return nil, err
	}
	value, err := domain.ParseMoney(amount)
	if err != nil {
		return nil, fmt.Errorf("parse settlement amount: %w", err)
	}
	settlement.AmountMinor = value
	return settlement, nil
}

func scanSettlementWithCount(row pgx.Row) (*domain.Settlement, int64, error) {
	settlement := &domain.Settlement{}
	var amount string
	var total int64
	if err := row.Scan(
		&settlement.ID, &settlement.GroupID, &settlement.GroupName,
		&settlement.FromUser.UserID, &settlement.FromUser.Fullname, &settlement.FromUser.Email,
		&settlement.ToUser.UserID, &settlement.ToUser.Fullname, &settlement.ToUser.Email,
		&amount, &settlement.Currency, &settlement.SettledAt, &settlement.Note,
		&settlement.CreatedBy.UserID, &settlement.CreatedBy.Fullname, &settlement.CreatedBy.Email,
		&settlement.CreatedAt, &settlement.UpdatedAt, &settlement.DeletedAt, &total,
	); err != nil {
		return nil, 0, err
	}
	value, err := domain.ParseMoney(amount)
	if err != nil {
		return nil, 0, fmt.Errorf("parse settlement amount: %w", err)
	}
	settlement.AmountMinor = value
	return settlement, total, nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && strings.TrimSpace(pgErr.Code) == "23505"
}
