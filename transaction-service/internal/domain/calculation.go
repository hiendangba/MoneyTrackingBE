package domain

import (
	"errors"
	"math/big"
	"sort"
)

var (
	ErrEmptySplits         = errors.New("at least one split is required")
	ErrMixedSplitTypes     = errors.New("split types cannot be mixed")
	ErrInvalidSplitValue   = errors.New("split value must be positive")
	ErrFixedSplitTotal     = errors.New("fixed splits must equal transaction amount")
	ErrInvalidPaymentTotal = errors.New("payments must equal transaction amount")
	ErrUnbalancedNet       = errors.New("balances do not sum to zero")
)

type SplitAllocation struct {
	UserID     string
	SplitType  SplitType
	SplitValue int64
	ShareMinor int64
}

func AllocateShares(amountMinor int64, splits []SplitAllocation) ([]SplitAllocation, error) {
	if amountMinor <= 0 || len(splits) == 0 {
		return nil, ErrEmptySplits
	}
	splitType := splits[0].SplitType
	totalValue := big.NewInt(0)
	seen := make(map[string]struct{}, len(splits))
	for _, split := range splits {
		if split.UserID == "" || split.SplitValue <= 0 {
			return nil, ErrInvalidSplitValue
		}
		if _, exists := seen[split.UserID]; exists {
			return nil, ErrInvalidSplitValue
		}
		seen[split.UserID] = struct{}{}
		if split.SplitType != splitType {
			return nil, ErrMixedSplitTypes
		}
		totalValue.Add(totalValue, big.NewInt(split.SplitValue))
	}

	result := append([]SplitAllocation(nil), splits...)
	if splitType == SplitTypeFixed {
		if !totalValue.IsInt64() || totalValue.Int64() != amountMinor {
			return nil, ErrFixedSplitTotal
		}
		for i := range result {
			result[i].ShareMinor = result[i].SplitValue
		}
		return result, nil
	}
	if splitType != SplitTypeRatio || totalValue.Sign() <= 0 {
		return nil, ErrInvalidSplitValue
	}

	type remainder struct {
		index int
		value *big.Int
	}
	remainders := make([]remainder, 0, len(result))
	var allocated int64
	for i := range result {
		product := new(big.Int).Mul(big.NewInt(amountMinor), big.NewInt(result[i].SplitValue))
		quotient, remainderValue := new(big.Int), new(big.Int)
		quotient.QuoRem(product, totalValue, remainderValue)
		if !quotient.IsInt64() {
			return nil, ErrInvalidSplitValue
		}
		result[i].ShareMinor = quotient.Int64()
		allocated += result[i].ShareMinor
		remainders = append(remainders, remainder{index: i, value: remainderValue})
	}
	sort.SliceStable(remainders, func(i, j int) bool {
		return remainders[i].value.Cmp(remainders[j].value) > 0
	})
	for remaining := amountMinor - allocated; remaining > 0; remaining-- {
		result[remainders[(amountMinor-allocated-remaining)%int64(len(remainders))].index].ShareMinor++
	}
	return result, nil
}

func ValidatePayments(amountMinor int64, payments []GroupPayment) error {
	if amountMinor <= 0 || len(payments) == 0 {
		return ErrInvalidPaymentTotal
	}
	var total int64
	seen := make(map[string]struct{}, len(payments))
	for _, payment := range payments {
		if payment.User.UserID == "" || payment.AmountMinor <= 0 {
			return ErrInvalidPaymentTotal
		}
		if _, exists := seen[payment.User.UserID]; exists {
			return ErrInvalidPaymentTotal
		}
		seen[payment.User.UserID] = struct{}{}
		total += payment.AmountMinor
	}
	if total != amountMinor {
		return ErrInvalidPaymentTotal
	}
	return nil
}

func CalculateBalances(
	transactionType TransactionType,
	payments []GroupPayment,
	splits []GroupSplit,
	settlements []Settlement,
) ([]MemberBalance, error) {
	balances := make(map[string]*MemberBalance)
	ensure := func(user UserSnapshot) *MemberBalance {
		balance, exists := balances[user.UserID]
		if !exists {
			balance = &MemberBalance{User: user}
			balances[user.UserID] = balance
		}
		if balance.User.Fullname == "" {
			balance.User = user
		}
		return balance
	}
	for _, payment := range payments {
		balance := ensure(payment.User)
		balance.PaidMinor += payment.AmountMinor
		if transactionType == TransactionTypeIncome {
			balance.NetMinor -= payment.AmountMinor
		} else {
			balance.NetMinor += payment.AmountMinor
		}
	}
	for _, split := range splits {
		balance := ensure(split.User)
		balance.ShareMinor += split.ShareMinor
		if transactionType == TransactionTypeIncome {
			balance.NetMinor += split.ShareMinor
		} else {
			balance.NetMinor -= split.ShareMinor
		}
	}
	for _, settlement := range settlements {
		from := ensure(settlement.FromUser)
		to := ensure(settlement.ToUser)
		from.SettlementsSent += settlement.AmountMinor
		to.SettlementsReceived += settlement.AmountMinor
		from.NetMinor += settlement.AmountMinor
		to.NetMinor -= settlement.AmountMinor
	}

	result := make([]MemberBalance, 0, len(balances))
	var total int64
	for _, balance := range balances {
		total += balance.NetMinor
		result = append(result, *balance)
	}
	if total != 0 {
		return nil, ErrUnbalancedNet
	}
	sort.Slice(result, func(i, j int) bool { return result[i].User.UserID < result[j].User.UserID })
	return result, nil
}

func SuggestSettlements(balances []MemberBalance, currency string) []SettlementSuggestion {
	type position struct {
		user   UserSnapshot
		amount int64
	}
	debtors := make([]position, 0)
	creditors := make([]position, 0)
	for _, balance := range balances {
		switch {
		case balance.NetMinor < 0:
			debtors = append(debtors, position{user: balance.User, amount: -balance.NetMinor})
		case balance.NetMinor > 0:
			creditors = append(creditors, position{user: balance.User, amount: balance.NetMinor})
		}
	}
	sort.Slice(debtors, func(i, j int) bool { return debtors[i].amount > debtors[j].amount })
	sort.Slice(creditors, func(i, j int) bool { return creditors[i].amount > creditors[j].amount })

	suggestions := make([]SettlementSuggestion, 0)
	for debtorIndex, creditorIndex := 0, 0; debtorIndex < len(debtors) && creditorIndex < len(creditors); {
		amount := min(debtors[debtorIndex].amount, creditors[creditorIndex].amount)
		suggestions = append(suggestions, SettlementSuggestion{
			FromUser:    debtors[debtorIndex].user,
			ToUser:      creditors[creditorIndex].user,
			AmountMinor: amount,
			Currency:    currency,
		})
		debtors[debtorIndex].amount -= amount
		creditors[creditorIndex].amount -= amount
		if debtors[debtorIndex].amount == 0 {
			debtorIndex++
		}
		if creditors[creditorIndex].amount == 0 {
			creditorIndex++
		}
	}
	return suggestions
}
