package domain

import "testing"

func TestAllocateShares(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		amount  int64
		splits  []SplitAllocation
		want    []int64
		wantErr error
	}{
		{
			name:   "ratio distributes rounding remainder deterministically",
			amount: 10_00,
			splits: []SplitAllocation{
				{UserID: "a", SplitType: SplitTypeRatio, SplitValue: 1_000_000},
				{UserID: "b", SplitType: SplitTypeRatio, SplitValue: 1_000_000},
				{UserID: "c", SplitType: SplitTypeRatio, SplitValue: 1_000_000},
			},
			want: []int64{334, 333, 333},
		},
		{
			name:   "fixed preserves explicit amounts",
			amount: 10_00,
			splits: []SplitAllocation{
				{UserID: "a", SplitType: SplitTypeFixed, SplitValue: 4_00},
				{UserID: "b", SplitType: SplitTypeFixed, SplitValue: 6_00},
			},
			want: []int64{400, 600},
		},
		{
			name:   "fixed must match amount",
			amount: 10_00,
			splits: []SplitAllocation{
				{UserID: "a", SplitType: SplitTypeFixed, SplitValue: 9_00},
			},
			wantErr: ErrFixedSplitTotal,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			got, err := AllocateShares(test.amount, test.splits)
			if test.wantErr != nil {
				if err != test.wantErr {
					t.Fatalf("AllocateShares() error = %v, want %v", err, test.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("AllocateShares() unexpected error: %v", err)
			}
			for i := range got {
				if got[i].ShareMinor != test.want[i] {
					t.Errorf("share[%d] = %d, want %d", i, got[i].ShareMinor, test.want[i])
				}
			}
		})
	}
}

func TestCalculateBalancesAndSuggestions(t *testing.T) {
	t.Parallel()

	alice := UserSnapshot{UserID: "alice", Fullname: "Alice"}
	bob := UserSnapshot{UserID: "bob", Fullname: "Bob"}
	carol := UserSnapshot{UserID: "carol", Fullname: "Carol"}
	balances, err := CalculateBalances(
		TransactionTypeExpense,
		[]GroupPayment{{User: alice, AmountMinor: 9_00}},
		[]GroupSplit{
			{User: alice, ShareMinor: 3_00},
			{User: bob, ShareMinor: 3_00},
			{User: carol, ShareMinor: 3_00},
		},
		nil,
	)
	if err != nil {
		t.Fatalf("CalculateBalances() unexpected error: %v", err)
	}
	suggestions := SuggestSettlements(balances, "VND")
	if len(suggestions) != 2 {
		t.Fatalf("SuggestSettlements() length = %d, want 2", len(suggestions))
	}
	for _, suggestion := range suggestions {
		if suggestion.ToUser.UserID != "alice" || suggestion.AmountMinor != 3_00 {
			t.Errorf("unexpected suggestion: %+v", suggestion)
		}
	}
}

func TestParseAndFormatMoney(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		raw     string
		want    int64
		wantErr bool
	}{
		{name: "integer", raw: "123", want: 12300},
		{name: "one decimal", raw: "123.4", want: 12340},
		{name: "two decimals", raw: "0.09", want: 9},
		{name: "too precise", raw: "1.001", wantErr: true},
		{name: "negative", raw: "-1", wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			got, err := ParseMoney(test.raw)
			if (err != nil) != test.wantErr {
				t.Fatalf("ParseMoney() error = %v, wantErr %v", err, test.wantErr)
			}
			if got != test.want {
				t.Errorf("ParseMoney() = %d, want %d", got, test.want)
			}
			if err == nil && FormatMoney(got) == "" {
				t.Error("FormatMoney() returned empty string")
			}
		})
	}
}
