package service

import (
	"time"

	"github.com/hiendangba/MoneyTrackingBE/Backend/transaction-service/internal/domain"
)

type CreateCategoryInput struct {
	Code        string
	Name        string
	Description string
}

type CreatePersonalInput struct {
	UserID          string
	CategoryID      string
	Type            domain.TransactionType
	Title           string
	Amount          string
	Currency        string
	TransactionDate time.Time
	Note            string
}

type PaymentInput struct {
	UserID string
	Amount string
	Note   string
}

type SplitInput struct {
	UserID     string
	SplitType  domain.SplitType
	SplitValue string
}

type CreateGroupInput struct {
	GroupID         string
	CategoryID      string
	Type            domain.TransactionType
	Title           string
	Amount          string
	Currency        string
	TransactionDate time.Time
	Note            string
	CreatedBy       string
	Payments        []PaymentInput
	Splits          []SplitInput
}

type CreateSettlementInput struct {
	GroupID    string
	FromUserID string
	ToUserID   string
	Amount     string
	Currency   string
	SettledAt  time.Time
	Note       string
	CreatedBy  string
}
