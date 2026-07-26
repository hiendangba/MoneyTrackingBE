package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"auth-service/internal/config"
	"auth-service/internal/domain"
	"auth-service/internal/dto"
	apperrors "auth-service/internal/errors"
	"auth-service/internal/infrastructure"
	"auth-service/internal/repository"
	"auth-service/internal/utils"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

type AuthService struct {
	cfg         config.AuthConfig
	jwtCfg      config.JWTConfig
	userRepo    repository.UserRepository
	jwtService  *JWTService
	redisClient *redis.Client
	publisher   infrastructure.OTPPublisher
}

func NewAuthService(
	cfg config.AuthConfig,
	userRepo repository.UserRepository,
	jwtService *JWTService,
	redisClient *redis.Client,
	publisher infrastructure.OTPPublisher,
) *AuthService {
	return &AuthService{
		cfg:         cfg,
		jwtCfg:      jwtService.cfg,
		userRepo:    userRepo,
		jwtService:  jwtService,
		redisClient: redisClient,
		publisher:   publisher,
	}
}

func (s *AuthService) Register(ctx context.Context, req dto.RegisterRequest) error {
	req.Email = utils.NormalizeEmail(req.Email)
	if err := utils.ValidateEmail(req.Email); err != nil {
		return apperrors.Validation(err.Error())
	}
	if err := utils.ValidatePassword(req.Password); err != nil {
		return apperrors.Validation(err.Error())
	}
	if strings.TrimSpace(req.Fullname) == "" {
		return apperrors.Validation("fullname is required")
	}

	if _, err := s.userRepo.FindByEmail(ctx, req.Email); err == nil {
		return apperrors.ErrEmailAlreadyExists
	} else if !errors.Is(err, apperrors.ErrUserNotFound) {
		return fmt.Errorf("check existing user: %w", err)
	}

	otp, err := utils.GenerateOTP(6)
	if err != nil {
		return fmt.Errorf("generate otp: %w", err)
	}
	otpHash, err := utils.HashOTP(otp)
	if err != nil {
		return fmt.Errorf("hash otp: %w", err)
	}

	payload := domain.OTPPayload{
		Email:        req.Email,
		Purpose:      domain.OTPPurposeRegister,
		OTPHash:      otpHash,
		ExpiresAt:    time.Now().Add(s.cfg.OTPTTL),
		AttemptCount: 0,
		RegisterPayload: &domain.RegisterDraftPayload{
			Fullname: req.Fullname,
			Email:    req.Email,
			Password: req.Password,
		},
	}
	if err := s.storeOTPPayload(ctx, s.registerOTPKey(req.Email), payload); err != nil {
		return err
	}

	if err := s.publisher.PublishRegisterOTP(ctx, req.Email, req.Fullname, otp, s.cfg.OTPTTL); err != nil {
		_ = s.redisClient.Del(ctx, s.registerOTPKey(req.Email)).Err()
		return fmt.Errorf("publish register otp: %w", err)
	}

	return nil
}

func (s *AuthService) VerifyOTP(ctx context.Context, req dto.VerifyOTPRequest, w http.ResponseWriter) error {
	req.Email = utils.NormalizeEmail(req.Email)
	if err := utils.ValidateEmail(req.Email); err != nil {
		return apperrors.Validation(err.Error())
	}
	if err := utils.ValidateOTP(req.OTP); err != nil {
		return apperrors.Validation(err.Error())
	}
	if req.Purpose != domain.OTPPurposeRegister {
		return apperrors.Validation("unsupported otp purpose")
	}

	payload, err := s.loadOTPPayload(ctx, s.registerOTPKey(req.Email))
	if err != nil {
		return err
	}
	if payload.Purpose != domain.OTPPurposeRegister || payload.RegisterPayload == nil {
		return apperrors.ErrInvalidOTP
	}

	if err := s.verifyOTPPayload(ctx, s.registerOTPKey(req.Email), payload, req.OTP); err != nil {
		return err
	}

	roleID, err := s.userRepo.FindRoleIDByCode(ctx, s.cfg.DefaultRoleCode)
	if err != nil {
		return fmt.Errorf("find default role: %w", err)
	}

	passwordHash, err := utils.HashPassword(payload.RegisterPayload.Password)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}
	userID, err := uuid.NewV7()
	if err != nil {
		return fmt.Errorf("generate user id: %w", err)
	}
	now := time.Now()
	user, err := s.userRepo.Create(ctx, domain.User{
		ID:           userID.String(),
		Fullname:     payload.RegisterPayload.Fullname,
		Email:        payload.RegisterPayload.Email,
		PasswordHash: passwordHash,
		RoleID:       roleID,
		IsActive:     true,
		CreatedAt:    now,
		UpdatedAt:    now,
	})
	if err != nil {
		return fmt.Errorf("create user: %w", err)
	}

	if err := s.redisClient.Del(ctx, s.registerOTPKey(req.Email)).Err(); err != nil {
		return fmt.Errorf("delete otp payload: %w", err)
	}

	if err := s.issueSessionCookies(w, user); err != nil {
		return err
	}
	return nil
}

func (s *AuthService) Login(ctx context.Context, req dto.LoginRequest, w http.ResponseWriter) error {
	user, err := s.authenticate(ctx, req)
	if err != nil {
		return err
	}

	return s.issueSessionCookies(w, user)
}

func (s *AuthService) MobileLogin(ctx context.Context, req dto.LoginRequest) (*dto.TokenResponse, error) {
	user, err := s.authenticate(ctx, req)
	if err != nil {
		return nil, err
	}

	sessionID, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("generate session id: %w", err)
	}
	return s.issueTokenPair(user, sessionID.String())
}

func (s *AuthService) authenticate(ctx context.Context, req dto.LoginRequest) (*domain.User, error) {
	req.Email = utils.NormalizeEmail(req.Email)
	user, err := s.userRepo.FindByEmail(ctx, req.Email)
	if err != nil {
		if errors.Is(err, apperrors.ErrUserNotFound) {
			return nil, apperrors.ErrInvalidCredentials
		}
		return nil, fmt.Errorf("find user by email: %w", err)
	}

	if err := utils.ComparePassword(user.PasswordHash, req.Password); err != nil {
		return nil, apperrors.ErrInvalidCredentials
	}
	if err := validateActiveUser(user); err != nil {
		return nil, err
	}
	return user, nil
}

func (s *AuthService) RefreshToken(ctx context.Context, r *http.Request, w http.ResponseWriter) error {
	refreshToken := utils.GetCookieValue(r, s.jwtCfg.RefreshCookieName)
	if refreshToken == "" {
		s.clearSessionCookies(w)
		return apperrors.ErrUnauthorized
	}

	tokens, err := s.rotateSession(ctx, refreshToken)
	if err != nil {
		s.clearSessionCookies(w)
		return err
	}
	s.setSessionCookies(w, tokens)
	return nil
}

func (s *AuthService) MobileRefresh(ctx context.Context, refreshToken string) (*dto.TokenResponse, error) {
	if strings.TrimSpace(refreshToken) == "" {
		return nil, apperrors.ErrUnauthorized
	}
	return s.rotateSession(ctx, refreshToken)
}

func (s *AuthService) Logout(ctx context.Context, r *http.Request, w http.ResponseWriter) error {
	refreshToken := utils.GetCookieValue(r, s.jwtCfg.RefreshCookieName)
	accessToken := s.extractAccessToken(r)
	err := s.revokeSessionFromTokens(ctx, accessToken, refreshToken)
	s.clearSessionCookies(w)
	return err
}

func (s *AuthService) MobileLogout(ctx context.Context, accessToken, refreshToken string) error {
	return s.revokeSessionFromTokens(ctx, accessToken, refreshToken)
}

func (s *AuthService) rotateSession(ctx context.Context, refreshToken string) (*dto.TokenResponse, error) {
	claims, err := s.jwtService.ParseRefreshToken(refreshToken)
	if err != nil {
		return nil, apperrors.ErrInvalidToken
	}
	if err := s.ensureSessionActive(ctx, claims.SessionID); err != nil {
		return nil, err
	}

	user, err := s.userRepo.FindByID(ctx, claims.Subject)
	if err != nil {
		return nil, fmt.Errorf("find refresh user: %w", err)
	}
	if err := validateActiveUser(user); err != nil {
		if revokeErr := s.revokeSession(ctx, claims.SessionID); revokeErr != nil {
			return nil, errors.Join(err, revokeErr)
		}
		return nil, err
	}
	if user.SessionVersion != claims.SessionVersion {
		if revokeErr := s.revokeSession(ctx, claims.SessionID); revokeErr != nil {
			return nil, errors.Join(apperrors.ErrSessionRevoked, revokeErr)
		}
		return nil, apperrors.ErrSessionRevoked
	}

	refreshTTL := s.jwtService.RemainingTTL(claims)
	if refreshTTL <= 0 {
		return nil, apperrors.ErrInvalidToken
	}
	consumed, err := s.redisClient.SetNX(
		ctx,
		s.usedRefreshKey(claims.ID),
		"used",
		refreshTTL,
	).Result()
	if err != nil {
		return nil, fmt.Errorf("consume refresh token: %w", err)
	}
	if !consumed {
		if revokeErr := s.revokeSession(ctx, claims.SessionID); revokeErr != nil {
			return nil, fmt.Errorf("revoke replayed session: %w", revokeErr)
		}
		return nil, apperrors.ErrSessionRevoked
	}

	return s.issueTokenPair(user, claims.SessionID)
}

func (s *AuthService) revokeSessionFromTokens(ctx context.Context, accessToken, refreshToken string) error {
	accessToken = strings.TrimSpace(accessToken)
	refreshToken = strings.TrimSpace(refreshToken)
	if accessToken == "" && refreshToken == "" {
		return nil
	}

	var refreshClaims *domain.TokenClaims
	if refreshToken != "" {
		var err error
		refreshClaims, err = s.jwtService.ParseRefreshToken(refreshToken)
		if err != nil {
			return apperrors.ErrInvalidToken
		}
	}

	var accessClaims *domain.TokenClaims
	if accessToken != "" {
		var err error
		accessClaims, err = s.jwtService.ParseAccessToken(accessToken)
		if err != nil && refreshClaims == nil {
			return apperrors.ErrInvalidToken
		}
	}

	if accessClaims != nil && refreshClaims != nil &&
		(accessClaims.SessionID != refreshClaims.SessionID || accessClaims.Subject != refreshClaims.Subject) {
		return apperrors.ErrInvalidToken
	}
	if refreshClaims != nil {
		return s.revokeSession(ctx, refreshClaims.SessionID)
	}
	return s.revokeSession(ctx, accessClaims.SessionID)
}

func (s *AuthService) ForgotPassword(ctx context.Context, req dto.ForgotPasswordRequest) error {
	req.Email = utils.NormalizeEmail(req.Email)
	if err := utils.ValidateEmail(req.Email); err != nil {
		return apperrors.Validation(err.Error())
	}

	user, err := s.userRepo.FindByEmail(ctx, req.Email)
	if err != nil {
		if errors.Is(err, apperrors.ErrUserNotFound) {
			return nil
		}
		return fmt.Errorf("find user by email: %w", err)
	}
	if err := validateActiveUser(user); err != nil {
		return nil
	}

	otp, err := utils.GenerateOTP(6)
	if err != nil {
		return fmt.Errorf("generate otp: %w", err)
	}
	otpHash, err := utils.HashOTP(otp)
	if err != nil {
		return fmt.Errorf("hash otp: %w", err)
	}

	payload := domain.OTPPayload{
		Email:        user.Email,
		Purpose:      domain.OTPPurposeReset,
		OTPHash:      otpHash,
		ExpiresAt:    time.Now().Add(s.cfg.OTPTTL),
		AttemptCount: 0,
	}

	if err := s.storeOTPPayload(ctx, s.resetOTPKey(req.Email), payload); err != nil {
		return err
	}

	if err := s.publisher.PublishResetOTP(ctx, req.Email, otp, s.cfg.OTPTTL); err != nil {
		_ = s.redisClient.Del(ctx, s.resetOTPKey(req.Email)).Err()
		return fmt.Errorf("publish reset otp: %w", err)
	}

	return nil
}

func (s *AuthService) ResetPassword(ctx context.Context, req dto.ResetPasswordRequest, w http.ResponseWriter) error {
	req.Email = utils.NormalizeEmail(req.Email)
	if err := utils.ValidateEmail(req.Email); err != nil {
		return apperrors.Validation(err.Error())
	}
	if err := utils.ValidateOTP(req.OTP); err != nil {
		return apperrors.Validation(err.Error())
	}
	if err := utils.ValidatePassword(req.NewPassword); err != nil {
		return apperrors.Validation(err.Error())
	}

	payload, err := s.loadOTPPayload(ctx, s.resetOTPKey(req.Email))
	if err != nil {
		return err
	}
	if payload.Purpose != domain.OTPPurposeReset {
		return apperrors.ErrInvalidOTP
	}
	if err := s.verifyOTPPayload(ctx, s.resetOTPKey(req.Email), payload, req.OTP); err != nil {
		return err
	}

	user, err := s.userRepo.FindByEmail(ctx, req.Email)
	if err != nil {
		return fmt.Errorf("find reset user: %w", err)
	}
	hash, err := utils.HashPassword(req.NewPassword)
	if err != nil {
		return fmt.Errorf("hash new password: %w", err)
	}
	if _, err := s.userRepo.UpdatePasswordAndIncrementSessionVersion(ctx, user.ID, hash); err != nil {
		return fmt.Errorf("update password: %w", err)
	}
	if err := s.redisClient.Del(ctx, s.resetOTPKey(req.Email)).Err(); err != nil {
		return fmt.Errorf("delete reset otp: %w", err)
	}

	s.clearSessionCookies(w)
	return nil
}

func (s *AuthService) ChangePassword(ctx context.Context, userID string, req dto.ChangePasswordRequest, w http.ResponseWriter) error {
	if err := utils.ValidatePassword(req.NewPassword); err != nil {
		return apperrors.Validation(err.Error())
	}

	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("find user by id: %w", err)
	}
	if err := utils.ComparePassword(user.PasswordHash, req.CurrentPassword); err != nil {
		return apperrors.ErrInvalidCredentials
	}

	hash, err := utils.HashPassword(req.NewPassword)
	if err != nil {
		return fmt.Errorf("hash new password: %w", err)
	}
	if _, err := s.userRepo.UpdatePasswordAndIncrementSessionVersion(ctx, userID, hash); err != nil {
		return fmt.Errorf("update password: %w", err)
	}

	s.clearSessionCookies(w)
	return nil
}

func (s *AuthService) Me(ctx context.Context, userID string) (*dto.MeResponse, error) {
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("find me: %w", err)
	}
	if err := validateActiveUser(user); err != nil {
		return nil, err
	}
	return &dto.MeResponse{
		ID:       user.ID,
		Fullname: user.Fullname,
		Email:    user.Email,
		RoleID:   user.RoleID,
		RoleCode: user.RoleCode,
	}, nil
}

func (s *AuthService) ValidateAccessToken(ctx context.Context, accessToken string) (*domain.TokenClaims, *domain.User, error) {
	claims, err := s.jwtService.ParseAccessToken(accessToken)
	if err != nil {
		return nil, nil, apperrors.ErrInvalidToken
	}

	user, err := s.userRepo.FindByID(ctx, claims.Subject)
	if err != nil {
		return nil, nil, fmt.Errorf("find access user: %w", err)
	}
	if err := validateActiveUser(user); err != nil {
		return nil, nil, err
	}
	if user.SessionVersion != claims.SessionVersion {
		return nil, nil, apperrors.ErrSessionRevoked
	}
	return claims, user, nil
}

func (s *AuthService) AccessCookieName() string {
	return s.jwtCfg.AccessCookieName
}

func (s *AuthService) extractAccessToken(r *http.Request) string {
	authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
	const bearer = "Bearer "
	if strings.HasPrefix(authHeader, bearer) {
		return strings.TrimSpace(strings.TrimPrefix(authHeader, bearer))
	}
	if token := utils.GetCookieValue(r, s.jwtCfg.AccessCookieName); token != "" {
		return token
	}
	return ""
}

func (s *AuthService) issueSessionCookies(w http.ResponseWriter, user *domain.User) error {
	sessionID, err := uuid.NewV7()
	if err != nil {
		return fmt.Errorf("generate session id: %w", err)
	}
	tokens, err := s.issueTokenPair(user, sessionID.String())
	if err != nil {
		return err
	}
	s.setSessionCookies(w, tokens)
	return nil
}

func (s *AuthService) issueTokenPair(user *domain.User, sessionID string) (*dto.TokenResponse, error) {
	accessToken, _, err := s.jwtService.GenerateAccessToken(
		user.ID,
		user.RoleID,
		user.RoleCode,
		sessionID,
		user.SessionVersion,
	)
	if err != nil {
		return nil, fmt.Errorf("generate access token: %w", err)
	}
	refreshToken, _, err := s.jwtService.GenerateRefreshToken(user.ID, sessionID, user.SessionVersion)
	if err != nil {
		return nil, fmt.Errorf("generate refresh token: %w", err)
	}

	return &dto.TokenResponse{
		AccessToken:      accessToken,
		RefreshToken:     refreshToken,
		TokenType:        "Bearer",
		ExpiresIn:        int64(s.jwtCfg.AccessTTL.Seconds()),
		RefreshExpiresIn: int64(s.jwtCfg.RefreshTTL.Seconds()),
	}, nil
}

func (s *AuthService) setSessionCookies(w http.ResponseWriter, tokens *dto.TokenResponse) {
	utils.SetTokenCookie(
		w,
		s.jwtCfg.AccessCookieName,
		tokens.AccessToken,
		s.jwtCfg.AccessTTL,
		s.cookieConfig("/"),
	)
	utils.SetTokenCookie(
		w,
		s.jwtCfg.RefreshCookieName,
		tokens.RefreshToken,
		s.jwtCfg.RefreshTTL,
		s.cookieConfig("/api/auth"),
	)
}

func (s *AuthService) ensureSessionActive(ctx context.Context, sessionID string) error {
	revoked, err := s.redisClient.Exists(ctx, s.revokedSessionKey(sessionID)).Result()
	if err != nil {
		return fmt.Errorf("check session revocation: %w", err)
	}
	if revoked > 0 {
		return apperrors.ErrSessionRevoked
	}
	return nil
}

func (s *AuthService) revokeSession(ctx context.Context, sessionID string) error {
	if strings.TrimSpace(sessionID) == "" {
		return apperrors.ErrInvalidToken
	}
	if err := s.redisClient.Set(ctx, s.revokedSessionKey(sessionID), "revoked", s.jwtCfg.RefreshTTL).Err(); err != nil {
		return fmt.Errorf("revoke session: %w", err)
	}
	return nil
}

func (s *AuthService) clearSessionCookies(w http.ResponseWriter) {
	utils.ClearTokenCookie(w, s.jwtCfg.AccessCookieName, s.cookieConfig("/"))
	utils.ClearTokenCookie(w, s.jwtCfg.RefreshCookieName, s.cookieConfig("/api/auth"))
}

func (s *AuthService) cookieConfig(path string) utils.CookieConfig {
	return utils.CookieConfig{
		Secure:   s.cfg.CookieSecure,
		SameSite: s.cfg.CookieSameSite,
		Domain:   s.cfg.CookieDomain,
		Path:     path,
	}
}

func validateActiveUser(user *domain.User) error {
	if user == nil || !user.IsActive || !user.RoleActive {
		return apperrors.ErrAccountInactive
	}
	return nil
}

func (s *AuthService) registerOTPKey(email string) string {
	return "auth:otp:register:" + email
}

func (s *AuthService) resetOTPKey(email string) string {
	return "auth:otp:reset:" + email
}

func (s *AuthService) usedRefreshKey(jti string) string {
	return "auth:refresh:used:" + jti
}

func (s *AuthService) revokedSessionKey(sessionID string) string {
	return "auth:session:revoked:" + sessionID
}

func (s *AuthService) storeOTPPayload(ctx context.Context, key string, payload domain.OTPPayload) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal otp payload: %w", err)
	}
	if err := s.redisClient.Set(ctx, key, body, s.cfg.OTPTTL).Err(); err != nil {
		return fmt.Errorf("store otp payload: %w", err)
	}
	return nil
}

func (s *AuthService) loadOTPPayload(ctx context.Context, key string) (*domain.OTPPayload, error) {
	raw, err := s.redisClient.Get(ctx, key).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, apperrors.ErrOTPExpired
		}
		return nil, fmt.Errorf("load otp payload: %w", err)
	}
	var payload domain.OTPPayload
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return nil, fmt.Errorf("unmarshal otp payload: %w", err)
	}
	return &payload, nil
}

func (s *AuthService) verifyOTPPayload(ctx context.Context, key string, payload *domain.OTPPayload, otp string) error {
	if time.Now().After(payload.ExpiresAt) {
		return apperrors.ErrOTPExpired
	}
	if payload.AttemptCount >= s.cfg.OTPMaxAttempts {
		return apperrors.ErrOTPAttemptsExceeded
	}
	if err := utils.CompareOTP(payload.OTPHash, otp); err != nil {
		payload.AttemptCount++
		_ = s.storeOTPPayload(ctx, key, *payload)
		return apperrors.ErrInvalidOTP
	}
	return nil
}
