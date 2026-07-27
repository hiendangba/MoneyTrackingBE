package httptransport

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	groupv1 "github.com/hiendangba/MoneyTrackingBE/Backend/group-service/gen/group/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func (g *Gateway) ListGroups(w http.ResponseWriter, r *http.Request, claims AuthClaims) {
	ctx, cancel := requestContext(r, g.cfg.RequestTimeout)
	defer cancel()

	includeInactive, _ := strconv.ParseBool(strings.TrimSpace(r.URL.Query().Get("include_inactive")))
	resp, err := g.groupClient.ListGroups(g.upstreamContext(ctx, r, &claims), &groupv1.ListGroupsRequest{
		UserId:          claims.UserID,
		IncludeInactive: includeInactive,
	})
	if err != nil {
		writeError(w, err, g.logger)
		return
	}

	groups := make([]Group, 0, len(resp.GetGroups()))
	for _, item := range resp.GetGroups() {
		groups = append(groups, toGroup(item))
	}
	writeJSON(w, http.StatusOK, groups)
}

func (g *Gateway) CreateGroup(w http.ResponseWriter, r *http.Request, claims AuthClaims) {
	var req CreateGroupRequest
	if err := g.decodeJSON(w, r, &req); err != nil {
		writeError(w, err, g.logger)
		return
	}
	ctx, cancel := requestContext(r, g.cfg.RequestTimeout)
	defer cancel()
	resp, err := g.groupClient.CreateGroup(g.upstreamContext(ctx, r, &claims), &groupv1.CreateGroupRequest{
		Name:          req.Name,
		Description:   req.Description,
		CreatorUserId: claims.UserID,
	})
	if err != nil {
		writeError(w, err, g.logger)
		return
	}
	writeJSON(w, http.StatusCreated, toGroup(resp))
}

func (g *Gateway) GetGroup(w http.ResponseWriter, r *http.Request, claims AuthClaims) {
	ctx, cancel := requestContext(r, g.cfg.RequestTimeout)
	defer cancel()
	resp, err := g.groupClient.GetGroup(g.upstreamContext(ctx, r, &claims), &groupv1.GetGroupRequest{
		Id: r.PathValue("id"),
	})
	if err != nil {
		writeError(w, err, g.logger)
		return
	}
	writeJSON(w, http.StatusOK, toGroup(resp))
}

func (g *Gateway) UpdateGroup(w http.ResponseWriter, r *http.Request, claims AuthClaims) {
	var req UpdateGroupRequest
	if err := g.decodeJSON(w, r, &req); err != nil {
		writeError(w, err, g.logger)
		return
	}
	ctx, cancel := requestContext(r, g.cfg.RequestTimeout)
	defer cancel()
	resp, err := g.groupClient.UpdateGroup(g.upstreamContext(ctx, r, &claims), &groupv1.UpdateGroupRequest{
		Id:          r.PathValue("id"),
		Name:        req.Name,
		Description: req.Description,
		IsActive:    req.IsActive,
	})
	if err != nil {
		writeError(w, err, g.logger)
		return
	}
	writeJSON(w, http.StatusOK, toGroup(resp))
}

func (g *Gateway) DeleteGroup(w http.ResponseWriter, r *http.Request, claims AuthClaims) {
	ctx, cancel := requestContext(r, g.cfg.RequestTimeout)
	defer cancel()
	resp, err := g.groupClient.DeleteGroup(g.upstreamContext(ctx, r, &claims), &groupv1.DeleteGroupRequest{
		Id: r.PathValue("id"),
	})
	if err != nil {
		writeError(w, err, g.logger)
		return
	}
	writeJSON(w, http.StatusOK, MessageResponse{Message: resp.GetMessage()})
}

func (g *Gateway) ListMembers(w http.ResponseWriter, r *http.Request, claims AuthClaims) {
	ctx, cancel := requestContext(r, g.cfg.RequestTimeout)
	defer cancel()
	resp, err := g.groupClient.ListMembers(g.upstreamContext(ctx, r, &claims), &groupv1.ListMembersRequest{
		GroupId: r.PathValue("id"),
	})
	if err != nil {
		writeError(w, err, g.logger)
		return
	}
	members := make([]GroupMember, 0, len(resp.GetMembers()))
	for _, item := range resp.GetMembers() {
		members = append(members, toGroupMember(item))
	}
	writeJSON(w, http.StatusOK, members)
}

func (g *Gateway) AddMember(w http.ResponseWriter, r *http.Request, claims AuthClaims) {
	var req AddMemberRequest
	if err := g.decodeJSON(w, r, &req); err != nil {
		writeError(w, err, g.logger)
		return
	}
	ctx, cancel := requestContext(r, g.cfg.RequestTimeout)
	defer cancel()
	resp, err := g.groupClient.AddMember(g.upstreamContext(ctx, r, &claims), &groupv1.AddMemberRequest{
		GroupId: r.PathValue("id"),
		UserId:  req.UserID,
		Role:    memberRoleFromString(req.Role),
	})
	if err != nil {
		writeError(w, err, g.logger)
		return
	}
	writeJSON(w, http.StatusCreated, toGroupMember(resp))
}

func (g *Gateway) RemoveMember(w http.ResponseWriter, r *http.Request, claims AuthClaims) {
	ctx, cancel := requestContext(r, g.cfg.RequestTimeout)
	defer cancel()
	resp, err := g.groupClient.RemoveMember(g.upstreamContext(ctx, r, &claims), &groupv1.RemoveMemberRequest{
		GroupId: r.PathValue("id"),
		UserId:  r.PathValue("user_id"),
	})
	if err != nil {
		writeError(w, err, g.logger)
		return
	}
	writeJSON(w, http.StatusOK, MessageResponse{Message: resp.GetMessage()})
}

func (g *Gateway) InviteMember(w http.ResponseWriter, r *http.Request, claims AuthClaims) {
	var req InviteMemberRequest
	if err := g.decodeJSON(w, r, &req); err != nil {
		writeError(w, err, g.logger)
		return
	}
	ctx, cancel := requestContext(r, g.cfg.RequestTimeout)
	defer cancel()
	resp, err := g.groupClient.InviteMember(g.upstreamContext(ctx, r, &claims), &groupv1.InviteMemberRequest{
		GroupId:       r.PathValue("id"),
		InvitedUserId: req.InvitedUserID,
		InvitedBy:     claims.UserID,
		Role:          memberRoleFromString(req.Role),
		ExpiredAt:     timePtrToProto(req.ExpiredAt),
	})
	if err != nil {
		writeError(w, err, g.logger)
		return
	}
	writeJSON(w, http.StatusCreated, toGroupInvitation(resp))
}

func (g *Gateway) ListInvitations(w http.ResponseWriter, r *http.Request, claims AuthClaims) {
	ctx, cancel := requestContext(r, g.cfg.RequestTimeout)
	defer cancel()
	resp, err := g.groupClient.ListInvitations(g.upstreamContext(ctx, r, &claims), &groupv1.ListInvitationsRequest{
		GroupId: r.PathValue("id"),
	})
	if err != nil {
		writeError(w, err, g.logger)
		return
	}
	invitations := make([]GroupInvitation, 0, len(resp.GetInvitations()))
	for _, item := range resp.GetInvitations() {
		invitations = append(invitations, toGroupInvitation(item))
	}
	writeJSON(w, http.StatusOK, invitations)
}

func (g *Gateway) RespondInvitation(w http.ResponseWriter, r *http.Request, claims AuthClaims) {
	var req RespondInvitationRequest
	if err := g.decodeJSON(w, r, &req); err != nil {
		writeError(w, err, g.logger)
		return
	}
	ctx, cancel := requestContext(r, g.cfg.RequestTimeout)
	defer cancel()
	resp, err := g.groupClient.RespondInvitation(g.upstreamContext(ctx, r, &claims), &groupv1.RespondInvitationRequest{
		InvitationId: r.PathValue("id"),
		Accepted:     req.Accepted,
	})
	if err != nil {
		writeError(w, err, g.logger)
		return
	}
	writeJSON(w, http.StatusOK, toGroupInvitation(resp))
}

func memberRoleFromString(raw string) groupv1.MemberRole {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "owner":
		return groupv1.MemberRole_MEMBER_ROLE_OWNER
	case "member", "":
		return groupv1.MemberRole_MEMBER_ROLE_MEMBER
	default:
		return groupv1.MemberRole_MEMBER_ROLE_UNSPECIFIED
	}
}

func memberRoleToString(role groupv1.MemberRole) string {
	switch role {
	case groupv1.MemberRole_MEMBER_ROLE_OWNER:
		return "owner"
	case groupv1.MemberRole_MEMBER_ROLE_MEMBER:
		return "member"
	default:
		return "unspecified"
	}
}

func invitationStatusToString(status groupv1.InvitationStatus) string {
	switch status {
	case groupv1.InvitationStatus_INVITATION_STATUS_PENDING:
		return "pending"
	case groupv1.InvitationStatus_INVITATION_STATUS_ACCEPTED:
		return "accepted"
	case groupv1.InvitationStatus_INVITATION_STATUS_REJECTED:
		return "rejected"
	case groupv1.InvitationStatus_INVITATION_STATUS_EXPIRED:
		return "expired"
	default:
		return "unspecified"
	}
}

func toGroup(resp *groupv1.Group) Group {
	if resp == nil {
		return Group{}
	}
	var description *string
	if resp.GetDescription() != "" {
		value := resp.GetDescription()
		description = &value
	}
	return Group{
		ID:          resp.GetId(),
		Name:        resp.GetName(),
		Description: description,
		IsActive:    resp.GetIsActive(),
		CreatedAt:   resp.GetCreatedAt().AsTime(),
		UpdatedAt:   resp.GetUpdatedAt().AsTime(),
	}
}

func toGroupMember(resp *groupv1.GroupMember) GroupMember {
	if resp == nil {
		return GroupMember{}
	}
	return GroupMember{
		GroupID:   resp.GetGroupId(),
		UserID:    resp.GetUserId(),
		Role:      memberRoleToString(resp.GetRole()),
		JoinedAt:  resp.GetJoinedAt().AsTime(),
		CreatedAt: resp.GetCreatedAt().AsTime(),
		UpdatedAt: resp.GetUpdatedAt().AsTime(),
	}
}

func toGroupInvitation(resp *groupv1.GroupInvitation) GroupInvitation {
	if resp == nil {
		return GroupInvitation{}
	}
	invitation := GroupInvitation{
		ID:            resp.GetId(),
		GroupID:       resp.GetGroupId(),
		InvitedUserID: resp.GetInvitedUserId(),
		InvitedBy:     resp.GetInvitedBy(),
		Role:          memberRoleToString(resp.GetRole()),
		Status:        invitationStatusToString(resp.GetStatus()),
		Token:         resp.GetToken(),
		CreatedAt:     resp.GetCreatedAt().AsTime(),
		UpdatedAt:     resp.GetUpdatedAt().AsTime(),
	}
	if ts := resp.GetExpiredAt(); ts != nil {
		value := ts.AsTime()
		invitation.ExpiredAt = &value
	}
	if ts := resp.GetRespondedAt(); ts != nil {
		value := ts.AsTime()
		invitation.RespondedAt = &value
	}
	return invitation
}

func timePtrToProto(value *time.Time) *timestamppb.Timestamp {
	if value == nil {
		return nil
	}
	return timestamppb.New(*value)
}
