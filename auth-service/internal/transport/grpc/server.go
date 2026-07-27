package grpctransport

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"time"

	authv1 "github.com/hiendangba/MoneyTrackingBE/Backend/auth-service/gen/auth/v1"
	"github.com/hiendangba/MoneyTrackingBE/Backend/auth-service/internal/dto"
	apperrors "github.com/hiendangba/MoneyTrackingBE/Backend/auth-service/internal/errors"
	"github.com/hiendangba/MoneyTrackingBE/Backend/auth-service/internal/service"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Server struct {
	authv1.UnimplementedAuthServiceServer
	authService *service.AuthService
	menuService *service.MenuService
	jwtService  *service.JWTService
	logger      *slog.Logger
}

func NewServer(authService *service.AuthService, menuService *service.MenuService, jwtService *service.JWTService, logger *slog.Logger) *Server {
	return &Server{authService: authService, menuService: menuService, jwtService: jwtService, logger: logger}
}

func (s *Server) Register(ctx context.Context, req *authv1.RegisterRequest) (*authv1.MessageResponse, error) {
	if err := s.authService.Register(ctx, dto.RegisterRequest{
		Fullname: req.GetFullname(),
		Email:    req.GetEmail(),
		Password: req.GetPassword(),
	}); err != nil {
		return nil, mapError(err)
	}
	return &authv1.MessageResponse{Message: "otp sent"}, nil
}

func (s *Server) VerifyOTP(ctx context.Context, req *authv1.VerifyOTPRequest) (*authv1.TokenResponse, error) {
	recorder := httptest.NewRecorder()
	if err := s.authService.VerifyOTP(ctx, dto.VerifyOTPRequest{
		Email:   req.GetEmail(),
		OTP:     req.GetOtp(),
		Purpose: req.GetPurpose(),
	}, recorder); err != nil {
		return nil, mapError(err)
	}
	cookies := recorder.Result().Cookies()
	return &authv1.TokenResponse{
		AccessToken:      cookieValue(cookies, s.authService.AccessCookieName()),
		RefreshToken:     cookieValue(cookies, s.authService.RefreshCookieName()),
		TokenType:        "Bearer",
		ExpiresIn:        int64(5 * time.Minute / time.Second),
		RefreshExpiresIn: int64(7 * 24 * time.Hour / time.Second),
	}, nil
}

func (s *Server) Login(ctx context.Context, req *authv1.LoginRequest) (*authv1.TokenResponse, error) {
	tokens, err := s.authService.MobileLogin(ctx, dto.LoginRequest{
		Email:    req.GetEmail(),
		Password: req.GetPassword(),
	})
	if err != nil {
		return nil, mapError(err)
	}
	return toProtoToken(tokens), nil
}

func (s *Server) RefreshToken(ctx context.Context, req *authv1.RefreshTokenRequest) (*authv1.TokenResponse, error) {
	tokens, err := s.authService.MobileRefresh(ctx, req.GetRefreshToken())
	if err != nil {
		return nil, mapError(err)
	}
	return toProtoToken(tokens), nil
}

func (s *Server) Logout(ctx context.Context, req *authv1.LogoutRequest) (*authv1.MessageResponse, error) {
	if err := s.authService.MobileLogout(ctx, req.GetAccessToken(), req.GetRefreshToken()); err != nil {
		return nil, mapError(err)
	}
	return &authv1.MessageResponse{Message: "logged out"}, nil
}

func (s *Server) ForgotPassword(ctx context.Context, req *authv1.ForgotPasswordRequest) (*authv1.MessageResponse, error) {
	if err := s.authService.ForgotPassword(ctx, dto.ForgotPasswordRequest{Email: req.GetEmail()}); err != nil {
		return nil, mapError(err)
	}
	return &authv1.MessageResponse{Message: "if the email exists, an otp has been sent"}, nil
}

func (s *Server) ResetPassword(ctx context.Context, req *authv1.ResetPasswordRequest) (*authv1.MessageResponse, error) {
	recorder := httptest.NewRecorder()
	if err := s.authService.ResetPassword(ctx, dto.ResetPasswordRequest{
		Email:       req.GetEmail(),
		OTP:         req.GetOtp(),
		NewPassword: req.GetNewPassword(),
	}, recorder); err != nil {
		return nil, mapError(err)
	}
	return &authv1.MessageResponse{Message: "password reset successfully"}, nil
}

func (s *Server) ChangePassword(ctx context.Context, req *authv1.ChangePasswordRequest) (*authv1.MessageResponse, error) {
	recorder := httptest.NewRecorder()
	if err := s.authService.ChangePassword(ctx, req.GetUserId(), dto.ChangePasswordRequest{
		CurrentPassword: req.GetCurrentPassword(),
		NewPassword:     req.GetNewPassword(),
	}, recorder); err != nil {
		return nil, mapError(err)
	}
	return &authv1.MessageResponse{Message: "password changed successfully"}, nil
}

func (s *Server) Me(ctx context.Context, req *authv1.MeRequest) (*authv1.MeResponse, error) {
	me, err := s.authService.Me(ctx, req.GetUserId())
	if err != nil {
		return nil, mapError(err)
	}
	return &authv1.MeResponse{
		Id:       me.ID,
		Fullname: me.Fullname,
		Email:    me.Email,
		RoleId:   me.RoleID,
		RoleCode: me.RoleCode,
	}, nil
}

func (s *Server) ValidateAccessToken(ctx context.Context, req *authv1.ValidateAccessTokenRequest) (*authv1.ValidateAccessTokenResponse, error) {
	claims, _, err := s.authService.ValidateAccessToken(ctx, req.GetAccessToken())
	if err != nil {
		return nil, mapError(err)
	}
	return &authv1.ValidateAccessTokenResponse{
		UserId:         claims.Subject,
		RoleId:         claims.RoleID,
		RoleCode:       claims.RoleCode,
		SessionId:      claims.SessionID,
		SessionVersion: claims.SessionVersion,
	}, nil
}

func (s *Server) ListMenus(ctx context.Context, _ *authv1.ListMenusRequest) (*authv1.ListMenusResponse, error) {
	menus, err := s.menuService.List(ctx)
	if err != nil {
		return nil, mapError(err)
	}
	response := &authv1.ListMenusResponse{Menus: make([]*authv1.Menu, 0, len(menus))}
	for _, menu := range menus {
		response.Menus = append(response.Menus, toProtoMenu(&menu))
	}
	return response, nil
}

func (s *Server) GetMenu(ctx context.Context, req *authv1.GetMenuRequest) (*authv1.Menu, error) {
	menu, err := s.menuService.GetByID(ctx, req.GetId())
	if err != nil {
		return nil, mapError(err)
	}
	return toProtoMenu(menu), nil
}

func (s *Server) TreeMenus(ctx context.Context, _ *authv1.TreeMenusRequest) (*authv1.TreeMenusResponse, error) {
	nodes, err := s.menuService.Tree(ctx)
	if err != nil {
		return nil, mapError(err)
	}
	response := &authv1.TreeMenusResponse{Nodes: make([]*authv1.MenuNode, 0, len(nodes))}
	for _, node := range nodes {
		response.Nodes = append(response.Nodes, toProtoMenuNode(node))
	}
	return response, nil
}

func (s *Server) GetJWKS(ctx context.Context, _ *authv1.GetJWKSRequest) (*authv1.GetJWKSResponse, error) {
	if s.jwtService == nil {
		return nil, status.Error(codes.Internal, "jwks service unavailable")
	}
	return &authv1.GetJWKSResponse{JwksJson: string(s.jwtService.JWKS())}, nil
}

func (s *Server) CreateMenu(ctx context.Context, req *authv1.CreateMenuRequest) (*authv1.Menu, error) {
	menu, err := s.menuService.Create(ctx, dto.CreateMenuRequest{
		Code:      req.GetCode(),
		Name:      req.GetName(),
		Route:     req.GetRoute(),
		Icon:      req.GetIcon(),
		ParentID:  req.GetParentId(),
		SortOrder: int(req.GetSortOrder()),
		IsActive:  req.IsActive,
	})
	if err != nil {
		return nil, mapError(err)
	}
	return toProtoMenu(menu), nil
}

func (s *Server) UpdateMenu(ctx context.Context, req *authv1.UpdateMenuRequest) (*authv1.Menu, error) {
	updated, err := s.menuService.Update(ctx, req.GetId(), dto.UpdateMenuRequest{
		Code:      req.Code,
		Name:      req.Name,
		Route:     req.Route,
		Icon:      req.Icon,
		ParentID:  req.ParentId,
		SortOrder: intPtrToInt(req.SortOrder),
		IsActive:  req.IsActive,
	})
	if err != nil {
		return nil, mapError(err)
	}
	return toProtoMenu(updated), nil
}

func (s *Server) DeleteMenu(ctx context.Context, req *authv1.DeleteMenuRequest) (*authv1.MessageResponse, error) {
	if err := s.menuService.Delete(ctx, req.GetId()); err != nil {
		return nil, mapError(err)
	}
	return &authv1.MessageResponse{Message: "menu deleted"}, nil
}

func cookieValue(cookies []*http.Cookie, name string) string {
	for _, cookie := range cookies {
		if cookie.Name == name {
			return cookie.Value
		}
	}
	return ""
}

func toProtoToken(tokens *dto.TokenResponse) *authv1.TokenResponse {
	if tokens == nil {
		return nil
	}
	return &authv1.TokenResponse{
		AccessToken:      tokens.AccessToken,
		RefreshToken:     tokens.RefreshToken,
		TokenType:        tokens.TokenType,
		ExpiresIn:        tokens.ExpiresIn,
		RefreshExpiresIn: tokens.RefreshExpiresIn,
	}
}

func toProtoMenu(menu *dto.MenuResponse) *authv1.Menu {
	if menu == nil {
		return nil
	}
	return &authv1.Menu{
		Id:        menu.ID,
		Code:      menu.Code,
		Name:      menu.Name,
		Route:     menu.Route,
		Icon:      deref(menu.Icon),
		ParentId:  deref(menu.ParentID),
		SortOrder: int32(menu.SortOrder),
		IsActive:  menu.IsActive,
	}
}

func toProtoMenuNode(node dto.MenuTreeResponse) *authv1.MenuNode {
	children := make([]*authv1.MenuNode, 0, len(node.Children))
	for _, child := range node.Children {
		children = append(children, toProtoMenuNode(child))
	}
	return &authv1.MenuNode{
		Id:        node.ID,
		Code:      node.Code,
		Name:      node.Name,
		Route:     node.Route,
		Icon:      deref(node.Icon),
		SortOrder: int32(node.SortOrder),
		Children:  children,
	}
}

func deref(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func intPtrToInt(value *int32) *int {
	if value == nil {
		return nil
	}
	v := int(*value)
	return &v
}

func mapError(err error) error {
	if err == nil {
		return nil
	}
	switch {
	case err == apperrors.ErrValidation:
		return status.Error(codes.InvalidArgument, err.Error())
	case err == apperrors.ErrInvalidCredentials:
		return status.Error(codes.Unauthenticated, err.Error())
	case err == apperrors.ErrEmailAlreadyExists:
		return status.Error(codes.AlreadyExists, err.Error())
	case err == apperrors.ErrUserNotFound:
		return status.Error(codes.NotFound, err.Error())
	case err == apperrors.ErrInvalidOTP:
		return status.Error(codes.Unauthenticated, err.Error())
	case err == apperrors.ErrOTPExpired:
		return status.Error(codes.FailedPrecondition, err.Error())
	case err == apperrors.ErrOTPAttemptsExceeded:
		return status.Error(codes.FailedPrecondition, err.Error())
	case err == apperrors.ErrInvalidToken:
		return status.Error(codes.Unauthenticated, err.Error())
	case err == apperrors.ErrUnauthorized:
		return status.Error(codes.Unauthenticated, err.Error())
	case err == apperrors.ErrMenuNotFound:
		return status.Error(codes.NotFound, err.Error())
	case err == apperrors.ErrMenuHasChildren:
		return status.Error(codes.FailedPrecondition, err.Error())
	case err == apperrors.ErrMenuCycle:
		return status.Error(codes.FailedPrecondition, err.Error())
	case err == apperrors.ErrConflict:
		return status.Error(codes.AlreadyExists, err.Error())
	case err == apperrors.ErrForbidden:
		return status.Error(codes.PermissionDenied, err.Error())
	case err == apperrors.ErrTooManyRequests:
		return status.Error(codes.ResourceExhausted, err.Error())
	case err == apperrors.ErrAccountInactive:
		return status.Error(codes.FailedPrecondition, err.Error())
	case err == apperrors.ErrSessionRevoked:
		return status.Error(codes.Unauthenticated, err.Error())
	default:
		return status.Error(codes.Internal, err.Error())
	}
}
