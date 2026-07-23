package config

import "testing"

func TestParseSender(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		input       string
		wantHeader  string
		wantAddress string
		wantErr     bool
	}{
		{
			name:        "address only",
			input:       "no-reply@example.com",
			wantHeader:  "<no-reply@example.com>",
			wantAddress: "no-reply@example.com",
		},
		{
			name:        "display name",
			input:       "Money Tracking <no-reply@example.com>",
			wantHeader:  "\"Money Tracking\" <no-reply@example.com>",
			wantAddress: "no-reply@example.com",
		},
		{
			name:    "invalid mailbox",
			input:   "Money Tracking",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			gotHeader, gotAddress, err := parseSender(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected an error")
				}
				return
			}
			if err != nil {
				t.Fatalf("parseSender() error = %v", err)
			}
			if gotHeader != tt.wantHeader {
				t.Fatalf("header = %q, want %q", gotHeader, tt.wantHeader)
			}
			if gotAddress != tt.wantAddress {
				t.Fatalf("address = %q, want %q", gotAddress, tt.wantAddress)
			}
		})
	}
}
