package clients

import (
	"context"
	"fmt"
	"time"

	authv1 "github.com/hiendangba/MoneyTrackingBE/Backend/auth-service/gen/auth/v1"
	groupv1 "github.com/hiendangba/MoneyTrackingBE/Backend/group-service/gen/group/v1"
	transactionv1 "github.com/hiendangba/MoneyTrackingBE/Backend/transaction-service/gen/transaction/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Clients struct {
	authConn        *grpc.ClientConn
	groupConn       *grpc.ClientConn
	transactionConn *grpc.ClientConn

	Auth        authv1.AuthServiceClient
	Group       groupv1.GroupServiceClient
	Transaction transactionv1.TransactionServiceClient
}

func New(ctx context.Context, authAddr, groupAddr, transactionAddr string) (*Clients, error) {
	authConn, err := dial(ctx, authAddr)
	if err != nil {
		return nil, fmt.Errorf("dial auth-service: %w", err)
	}
	groupConn, err := dial(ctx, groupAddr)
	if err != nil {
		_ = authConn.Close()
		return nil, fmt.Errorf("dial group-service: %w", err)
	}
	transactionConn, err := dial(ctx, transactionAddr)
	if err != nil {
		_ = authConn.Close()
		_ = groupConn.Close()
		return nil, fmt.Errorf("dial transaction-service: %w", err)
	}

	return &Clients{
		authConn:        authConn,
		groupConn:       groupConn,
		transactionConn: transactionConn,
		Auth:            authv1.NewAuthServiceClient(authConn),
		Group:           groupv1.NewGroupServiceClient(groupConn),
		Transaction:     transactionv1.NewTransactionServiceClient(transactionConn),
	}, nil
}

func (c *Clients) Close() error {
	var firstErr error
	if c.authConn != nil {
		if err := c.authConn.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if c.groupConn != nil {
		if err := c.groupConn.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if c.transactionConn != nil {
		if err := c.transactionConn.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func dial(ctx context.Context, address string) (*grpc.ClientConn, error) {
	dialCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(
		dialCtx,
		address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		return nil, err
	}
	return conn, nil
}
