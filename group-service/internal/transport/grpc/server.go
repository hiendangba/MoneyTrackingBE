package grpctransport

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	groupv1 "group-service/gen/group/v1"
	"group-service/internal/domain"
	dto "group-service/internal/dto"
	apperrors "group-service/internal/errors"
	"group-service/internal/service"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Server struct {
	groupv1.UnimplementedGroupServiceServer
	svc    *service.GroupService
	logger *slog.Logger
}

func NewServer(svc *service.GroupService, logger *slog.Logger) *Server {
	return &Server{svc: svc, logger: logger}
}

func (s *Server) CreateGroup(ctx context.Context, req *groupv1.CreateGroupRequest) (*groupv1.Group, error) {
	group, err := s.svc.CreateGroup(ctx, dto.CreateGroupRequest{
		Name:          req.GetName(),
		Description:   req.GetDescription(),
		CreatorUserID: req.GetCreatorUserId(),
	})
	if err != nil {
		return nil, mapError(err)
	}
	return toProtoGroup(group), nil
}

func (s *Server) GetGroup(ctx context.Context, req *groupv1.GetGroupRequest) (*groupv1.Group, error) {
	group, err := s.svc.GetGroup(ctx, req.GetId())
	if err != nil {
		return nil, mapError(err)
	}
	return toProtoGroup(group), nil
}

func (s *Server) ListGroups(ctx context.Context, req *groupv1.ListGroupsRequest) (*groupv1.ListGroupsResponse, error) {
	groups, err := s.svc.ListGroups(ctx, req.GetUserId(), req.GetIncludeInactive())
	if err != nil {
		return nil, mapError(err)
	}
	response := &groupv1.ListGroupsResponse{Groups: make([]*groupv1.Group, 0, len(groups))}
	for _, group := range groups {
		response.Groups = append(response.Groups, toProtoGroup(&group))
	}
	return response, nil
}

func (s *Server) UpdateGroup(ctx context.Context, req *groupv1.UpdateGroupRequest) (*groupv1.Group, error) {
	updated, err := s.svc.UpdateGroup(ctx, req.GetId(), dto.UpdateGroupRequest{
		Name:        optionalString(req.Name),
		Description: optionalString(req.Description),
		IsActive:    req.IsActive,
	})
	if err != nil {
		return nil, mapError(err)
	}
	return toProtoGroup(updated), nil
}

func (s *Server) DeleteGroup(ctx context.Context, req *groupv1.DeleteGroupRequest) (*groupv1.DeleteGroupResponse, error) {
	if err := s.svc.DeleteGroup(ctx, req.GetId()); err != nil {
		return nil, mapError(err)
	}
	return &groupv1.DeleteGroupResponse{Message: "group deleted"}, nil
}

func (s *Server) ListMembers(ctx context.Context, req *groupv1.ListMembersRequest) (*groupv1.ListMembersResponse, error) {
	members, err := s.svc.ListMembers(ctx, req.GetGroupId())
	if err != nil {
		return nil, mapError(err)
	}
	response := &groupv1.ListMembersResponse{Members: make([]*groupv1.GroupMember, 0, len(members))}
	for _, member := range members {
		response.Members = append(response.Members, toProtoMember(&member))
	}
	return response, nil
}

func (s *Server) GetMembership(ctx context.Context, req *groupv1.GetMembershipRequest) (*groupv1.GroupMember, error) {
	member, err := s.svc.GetMembership(ctx, req.GetGroupId(), req.GetUserId())
	if err != nil {
		return nil, mapError(err)
	}
	return toProtoMember(member), nil
}

func (s *Server) AddMember(ctx context.Context, req *groupv1.AddMemberRequest) (*groupv1.GroupMember, error) {
	member, err := s.svc.AddMember(ctx, dto.AddMemberRequest{
		GroupID: req.GetGroupId(),
		UserID:  req.GetUserId(),
		Role:    string(memberRoleFromProto(req.GetRole())),
	})
	if err != nil {
		return nil, mapError(err)
	}
	return toProtoMember(member), nil
}

func (s *Server) RemoveMember(ctx context.Context, req *groupv1.RemoveMemberRequest) (*groupv1.DeleteGroupMemberResponse, error) {
	if err := s.svc.RemoveMember(ctx, req.GetGroupId(), req.GetUserId()); err != nil {
		return nil, mapError(err)
	}
	return &groupv1.DeleteGroupMemberResponse{Message: "member removed"}, nil
}

func (s *Server) InviteMember(ctx context.Context, req *groupv1.InviteMemberRequest) (*groupv1.GroupInvitation, error) {
	invitation, err := s.svc.InviteMember(ctx, dto.InviteMemberRequest{
		GroupID:       req.GetGroupId(),
		InvitedUserID: req.GetInvitedUserId(),
		InvitedBy:     req.GetInvitedBy(),
		Role:          string(memberRoleFromProto(req.GetRole())),
		ExpiredAt:     timestampOrNil(req.ExpiredAt),
	})
	if err != nil {
		return nil, mapError(err)
	}
	return toProtoInvitation(invitation), nil
}

func (s *Server) ListInvitations(ctx context.Context, req *groupv1.ListInvitationsRequest) (*groupv1.ListInvitationsResponse, error) {
	invitations, err := s.svc.ListInvitations(ctx, req.GetGroupId())
	if err != nil {
		return nil, mapError(err)
	}
	response := &groupv1.ListInvitationsResponse{Invitations: make([]*groupv1.GroupInvitation, 0, len(invitations))}
	for _, invitation := range invitations {
		response.Invitations = append(response.Invitations, toProtoInvitation(&invitation))
	}
	return response, nil
}

func (s *Server) RespondInvitation(ctx context.Context, req *groupv1.RespondInvitationRequest) (*groupv1.GroupInvitation, error) {
	invitation, err := s.svc.RespondInvitation(ctx, req.GetInvitationId(), req.GetAccepted())
	if err != nil {
		return nil, mapError(err)
	}
	return toProtoInvitation(invitation), nil
}

func toProtoGroup(group *domain.Group) *groupv1.Group {
	if group == nil {
		return nil
	}
	return &groupv1.Group{
		Id:          group.ID,
		Name:        group.Name,
		Description: deref(group.Description),
		IsActive:    group.IsActive,
		CreatedAt:   timestamppb.New(group.CreatedAt),
		UpdatedAt:   timestamppb.New(group.UpdatedAt),
	}
}

func toProtoMember(member *domain.GroupMember) *groupv1.GroupMember {
	if member == nil {
		return nil
	}
	return &groupv1.GroupMember{
		GroupId:   member.GroupID,
		UserId:    member.UserID,
		Role:      memberRoleToProto(member.Role),
		JoinedAt:  timestamppb.New(member.JoinedAt),
		CreatedAt: timestamppb.New(member.CreatedAt),
		UpdatedAt: timestamppb.New(member.UpdatedAt),
	}
}

func toProtoInvitation(invitation *domain.GroupInvitation) *groupv1.GroupInvitation {
	if invitation == nil {
		return nil
	}
	resp := &groupv1.GroupInvitation{
		Id:            invitation.ID,
		GroupId:       invitation.GroupID,
		InvitedUserId: invitation.InvitedUserID,
		InvitedBy:     invitation.InvitedBy,
		Role:          memberRoleToProto(invitation.Role),
		Status:        invitationStatusToProto(invitation.Status),
		Token:         invitation.Token,
		CreatedAt:     timestamppb.New(invitation.CreatedAt),
		UpdatedAt:     timestamppb.New(invitation.UpdatedAt),
	}
	if invitation.ExpiredAt != nil {
		resp.ExpiredAt = timestamppb.New(*invitation.ExpiredAt)
	}
	if invitation.RespondedAt != nil {
		resp.RespondedAt = timestamppb.New(*invitation.RespondedAt)
	}
	return resp
}

func optionalString(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	return &trimmed
}

func deref(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func memberRoleToProto(role domain.MemberRole) groupv1.MemberRole {
	switch role {
	case domain.MemberRoleOwner:
		return groupv1.MemberRole_MEMBER_ROLE_OWNER
	case domain.MemberRoleMember:
		return groupv1.MemberRole_MEMBER_ROLE_MEMBER
	default:
		return groupv1.MemberRole_MEMBER_ROLE_UNSPECIFIED
	}
}

func memberRoleFromProto(role groupv1.MemberRole) domain.MemberRole {
	switch role {
	case groupv1.MemberRole_MEMBER_ROLE_OWNER:
		return domain.MemberRoleOwner
	case groupv1.MemberRole_MEMBER_ROLE_MEMBER:
		return domain.MemberRoleMember
	default:
		return domain.MemberRoleMember
	}
}

func invitationStatusToProto(status domain.InvitationStatus) groupv1.InvitationStatus {
	switch status {
	case domain.InvitationStatusPending:
		return groupv1.InvitationStatus_INVITATION_STATUS_PENDING
	case domain.InvitationStatusAccepted:
		return groupv1.InvitationStatus_INVITATION_STATUS_ACCEPTED
	case domain.InvitationStatusRejected:
		return groupv1.InvitationStatus_INVITATION_STATUS_REJECTED
	case domain.InvitationStatusExpired:
		return groupv1.InvitationStatus_INVITATION_STATUS_EXPIRED
	default:
		return groupv1.InvitationStatus_INVITATION_STATUS_UNSPECIFIED
	}
}

func timestampOrNil(ts *timestamppb.Timestamp) *time.Time {
	if ts == nil {
		return nil
	}
	value := ts.AsTime()
	return &value
}

func mapError(err error) error {
	if err == nil {
		return nil
	}

	switch apperrors.StatusCode(err) {
	case 400:
		switch {
		case errors.Is(err, apperrors.ErrValidation):
			return status.Error(codes.InvalidArgument, err.Error())
		case errors.Is(err, apperrors.ErrGroupHasMembers):
			return status.Error(codes.FailedPrecondition, err.Error())
		case errors.Is(err, apperrors.ErrGroupInactive):
			return status.Error(codes.FailedPrecondition, err.Error())
		case errors.Is(err, apperrors.ErrInvitationExpired):
			return status.Error(codes.FailedPrecondition, err.Error())
		case errors.Is(err, apperrors.ErrInvitationNotPending):
			return status.Error(codes.FailedPrecondition, err.Error())
		default:
			return status.Error(codes.InvalidArgument, err.Error())
		}
	case 403:
		return status.Error(codes.PermissionDenied, err.Error())
	case 404:
		return status.Error(codes.NotFound, err.Error())
	case 409:
		return status.Error(codes.AlreadyExists, err.Error())
	default:
		return status.Error(codes.Internal, err.Error())
	}
}
