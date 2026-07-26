package clients

import (
	"context"
	"fmt"
	"time"

	authv1 "auth-service/gen/auth/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	groupv1 "group-service/gen/group/v1"
)

type Clients struct {
	authConn  *grpc.ClientConn
	groupConn *grpc.ClientConn

	Auth  authv1.AuthServiceClient
	Group groupv1.GroupServiceClient
}

func New(ctx context.Context, authAddr, groupAddr string) (*Clients, error) {
	authConn, err := dial(ctx, authAddr)
	if err != nil {
		return nil, fmt.Errorf("dial auth-service: %w", err)
	}
	groupConn, err := dial(ctx, groupAddr)
	if err != nil {
		_ = authConn.Close()
		return nil, fmt.Errorf("dial group-service: %w", err)
	}

	return &Clients{
		authConn:  authConn,
		groupConn: groupConn,
		Auth:      authv1.NewAuthServiceClient(authConn),
		Group:     groupv1.NewGroupServiceClient(groupConn),
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
