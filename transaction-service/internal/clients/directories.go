package clients

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	authv1 "github.com/hiendangba/MoneyTrackingBE/Backend/auth-service/gen/auth/v1"
	groupv1 "github.com/hiendangba/MoneyTrackingBE/Backend/group-service/gen/group/v1"
	"github.com/hiendangba/MoneyTrackingBE/Backend/transaction-service/internal/domain"
	apperrors "github.com/hiendangba/MoneyTrackingBE/Backend/transaction-service/internal/errors"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

type GroupSnapshot struct {
	ID       string
	Name     string
	IsActive bool
}

type GroupDirectory interface {
	GetGroup(ctx context.Context, groupID string) (GroupSnapshot, error)
	ListMemberIDs(ctx context.Context, groupID string) ([]string, error)
	GetMemberRole(ctx context.Context, groupID, userID string) (string, error)
}

type UserDirectory interface {
	GetUsers(ctx context.Context, userIDs []string) (map[string]domain.UserSnapshot, error)
}

type Clients struct {
	authConn  *grpc.ClientConn
	groupConn *grpc.ClientConn
	Auth      *AuthDirectory
	Group     *GRPCGroupDirectory
}

func New(ctx context.Context, authAddr, groupAddr string, timeout time.Duration) (*Clients, error) {
	authConn, err := dial(ctx, authAddr, timeout)
	if err != nil {
		return nil, fmt.Errorf("dial auth-service: %w", err)
	}
	groupConn, err := dial(ctx, groupAddr, timeout)
	if err != nil {
		_ = authConn.Close()
		return nil, fmt.Errorf("dial group-service: %w", err)
	}
	return &Clients{
		authConn:  authConn,
		groupConn: groupConn,
		Auth: &AuthDirectory{
			client:  authv1.NewAuthServiceClient(authConn),
			timeout: timeout,
		},
		Group: &GRPCGroupDirectory{
			client:  groupv1.NewGroupServiceClient(groupConn),
			timeout: timeout,
		},
	}, nil
}

func (c *Clients) Close() error {
	var errs []error
	if c.authConn != nil {
		errs = append(errs, c.authConn.Close())
	}
	if c.groupConn != nil {
		errs = append(errs, c.groupConn.Close())
	}
	return errors.Join(errs...)
}

func dial(ctx context.Context, address string, timeout time.Duration) (*grpc.ClientConn, error) {
	dialCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return grpc.DialContext(
		dialCtx,
		address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
}

type AuthDirectory struct {
	client  authv1.AuthServiceClient
	timeout time.Duration
}

func (d *AuthDirectory) GetUsers(ctx context.Context, userIDs []string) (map[string]domain.UserSnapshot, error) {
	unique := make(map[string]struct{}, len(userIDs))
	for _, userID := range userIDs {
		userID = strings.TrimSpace(userID)
		if userID == "" {
			return nil, apperrors.Validation("user_id is required")
		}
		unique[userID] = struct{}{}
	}
	result := make(map[string]domain.UserSnapshot, len(unique))
	for userID := range unique {
		callCtx, cancel := context.WithTimeout(ctx, d.timeout)
		response, err := d.client.Me(callCtx, &authv1.MeRequest{UserId: userID})
		cancel()
		if err != nil {
			return nil, mapUpstreamError("auth-service", err)
		}
		result[userID] = domain.UserSnapshot{
			UserID:   response.GetId(),
			Fullname: response.GetFullname(),
			Email:    response.GetEmail(),
		}
	}
	return result, nil
}

type GRPCGroupDirectory struct {
	client  groupv1.GroupServiceClient
	timeout time.Duration
}

func (d *GRPCGroupDirectory) GetGroup(ctx context.Context, groupID string) (GroupSnapshot, error) {
	callCtx, cancel := context.WithTimeout(ctx, d.timeout)
	defer cancel()
	response, err := d.client.GetGroup(callCtx, &groupv1.GetGroupRequest{Id: groupID})
	if err != nil {
		return GroupSnapshot{}, mapUpstreamError("group-service", err)
	}
	return GroupSnapshot{ID: response.GetId(), Name: response.GetName(), IsActive: response.GetIsActive()}, nil
}

func (d *GRPCGroupDirectory) ListMemberIDs(ctx context.Context, groupID string) ([]string, error) {
	callCtx, cancel := context.WithTimeout(ctx, d.timeout)
	defer cancel()
	response, err := d.client.ListMembers(callCtx, &groupv1.ListMembersRequest{GroupId: groupID})
	if err != nil {
		return nil, mapUpstreamError("group-service", err)
	}
	ids := make([]string, 0, len(response.GetMembers()))
	for _, member := range response.GetMembers() {
		ids = append(ids, member.GetUserId())
	}
	return ids, nil
}

func (d *GRPCGroupDirectory) GetMemberRole(ctx context.Context, groupID, userID string) (string, error) {
	callCtx, cancel := context.WithTimeout(ctx, d.timeout)
	defer cancel()
	member, err := d.client.GetMembership(callCtx, &groupv1.GetMembershipRequest{
		GroupId: groupID,
		UserId:  userID,
	})
	if err != nil {
		return "", mapUpstreamError("group-service", err)
	}
	switch member.GetRole() {
	case groupv1.MemberRole_MEMBER_ROLE_OWNER:
		return "owner", nil
	case groupv1.MemberRole_MEMBER_ROLE_MEMBER:
		return "member", nil
	default:
		return "", apperrors.ErrForbidden
	}
}

func mapUpstreamError(service string, err error) error {
	switch status.Code(err) {
	case codes.InvalidArgument:
		return apperrors.Validation(err.Error())
	case codes.NotFound:
		return fmt.Errorf("%s: %w", service, apperrors.ErrNotFound)
	case codes.PermissionDenied:
		return apperrors.ErrForbidden
	case codes.Unavailable, codes.DeadlineExceeded:
		return fmt.Errorf("%s: %w", service, apperrors.ErrUpstreamUnavailable)
	default:
		return fmt.Errorf("%s call: %w", service, err)
	}
}
