package domain

import "time"

type TransactionType string

const (
	TransactionTypeIncome  TransactionType = "income"
	TransactionTypeExpense TransactionType = "expense"
)

type SplitType string

const (
	SplitTypeRatio SplitType = "ratio"
	SplitTypeFixed SplitType = "fixed"
)

type UserSnapshot struct {
	UserID   string
	Fullname string
	Email    string
}

type Category struct {
	ID          string
	Code        string
	Name        string
	Description *string
	IsActive    bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
	DeletedAt   *time.Time
}

type PersonalTransaction struct {
	ID              string
	User            UserSnapshot
	CategoryID      *string
	CategoryCode    *string
	CategoryName    *string
	Type            TransactionType
	Title           string
	AmountMinor     int64
	Currency        string
	TransactionDate time.Time
	Note            *string
	CreatedAt       time.Time
	UpdatedAt       time.Time
	DeletedAt       *time.Time
}

type GroupPayment struct {
	ID            string
	TransactionID string
	User          UserSnapshot
	AmountMinor   int64
	Note          *string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type GroupSplit struct {
	ID            string
	TransactionID string
	User          UserSnapshot
	SplitType     SplitType
	SplitValue    int64
	ShareMinor    int64
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type GroupTransaction struct {
	ID              string
	GroupID         string
	GroupName       string
	CategoryID      *string
	CategoryCode    *string
	CategoryName    *string
	Type            TransactionType
	Title           string
	AmountMinor     int64
	Currency        string
	TransactionDate time.Time
	Note            *string
	CreatedBy       UserSnapshot
	Payments        []GroupPayment
	Splits          []GroupSplit
	CreatedAt       time.Time
	UpdatedAt       time.Time
	DeletedAt       *time.Time
}

type Settlement struct {
	ID          string
	GroupID     string
	GroupName   string
	FromUser    UserSnapshot
	ToUser      UserSnapshot
	AmountMinor int64
	Currency    string
	SettledAt   time.Time
	Note        *string
	CreatedBy   UserSnapshot
	CreatedAt   time.Time
	UpdatedAt   time.Time
	DeletedAt   *time.Time
}

type MemberBalance struct {
	User                UserSnapshot
	PaidMinor           int64
	ShareMinor          int64
	SettlementsSent     int64
	SettlementsReceived int64
	NetMinor            int64
}

type SettlementSuggestion struct {
	FromUser    UserSnapshot
	ToUser      UserSnapshot
	AmountMinor int64
	Currency    string
}

type ListFilter struct {
	From       *time.Time
	To         *time.Time
	Type       TransactionType
	CategoryID string
	Limit      int
	Offset     int
}
