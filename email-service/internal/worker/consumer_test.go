package worker

import (
	"testing"

	amqp "github.com/rabbitmq/amqp091-go"
)

func TestMessageRetryCount(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		headers amqp.Table
		want    int
	}{
		{name: "missing", headers: nil, want: 0},
		{name: "int32", headers: amqp.Table{"x-email-retry-count": int32(2)}, want: 2},
		{name: "int64", headers: amqp.Table{"x-email-retry-count": int64(3)}, want: 3},
		{name: "unsupported", headers: amqp.Table{"x-email-retry-count": "3"}, want: 0},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := messageRetryCount(tt.headers); got != tt.want {
				t.Fatalf("messageRetryCount() = %d, want %d", got, tt.want)
			}
		})
	}
}
