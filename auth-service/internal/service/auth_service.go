package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
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
	logger      *slog.Logger
}

func NewAuthService(
	cfg config.AuthConfig,
	userRepo repository.UserRepository,
	jwtService *JWTService,
	redisClient *redis.Client,
	publisher infrastructure.OTPPublisher,
	logger *slog.Logger,
) *AuthService {
	return &AuthService{
		cfg:         cfg,
		jwtCfg:      jwtService.cfg,
		userRepo:    userRepo,
		jwtService:  jwtService,
		redisClient: redisClient,
		publisher:   publisher,
		logger:      logger,
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

	if err := s.issueSessionCookies(w, user.ID); err != nil {
		return err
	}
	return nil
}

func (s *AuthService) Login(ctx context.Context, req dto.LoginRequest, w http.ResponseWriter) error {
	req.Email = utils.NormalizeEmail(req.Email)
	user, err := s.userRepo.FindByEmail(ctx, req.Email)
	if err != nil {
		if errors.Is(err, apperrors.ErrUserNotFound) {
			return apperrors.ErrInvalidCredentials
		}
		return fmt.Errorf("find user by email: %w", err)
	}

	if err := utils.ComparePassword(user.PasswordHash, req.Password); err != nil {
		return apperrors.ErrInvalidCredentials
	}

	return s.issueSessionCookies(w, user.ID)
}

func (s *AuthService) RefreshToken(ctx context.Context, r *http.Request, w http.ResponseWriter) error {
	refreshToken := utils.GetCookieValue(r, s.jwtCfg.RefreshCookieName)
	if refreshToken == "" {
		s.clearSessionCookies(w)
		return apperrors.ErrUnauthorized
	}

	refreshClaims, err := s.jwtService.ParseAndValidate(refreshToken)
	if err != nil {
		s.clearSessionCookies(w)
		return apperrors.ErrInvalidToken
	}
	if err := ValidateTokenType(refreshClaims, domain.TokenTypeRefresh); err != nil {
		s.clearSessionCookies(w)
		return err
	}
	if err := s.ensureNotBlacklisted(ctx, domain.TokenTypeRefresh, refreshClaims.ID); err != nil {
		s.clearSessionCookies(w)
		return err
	}

	if err := s.blacklistTokenClaims(ctx, refreshClaims); err != nil {
		return err
	}

	if accessToken := s.extractAccessToken(r); accessToken != "" {
		if accessClaims, parseErr := s.jwtService.ParseAndValidate(accessToken); parseErr == nil {
			_ = s.blacklistTokenClaims(ctx, accessClaims)
		}
	}

	return s.issueSessionCookies(w, refreshClaims.Subject)
}

func (s *AuthService) Logout(ctx context.Context, r *http.Request, w http.ResponseWriter) error {
	if accessToken := s.extractAccessToken(r); accessToken != "" {
		if claims, err := s.jwtService.ParseAndValidate(accessToken); err == nil {
			_ = s.blacklistTokenClaims(ctx, claims)
		}
	}

	refreshToken := utils.GetCookieValue(r, s.jwtCfg.RefreshCookieName)
	if refreshToken != "" {
		if claims, err := s.jwtService.ParseAndValidate(refreshToken); err == nil {
			_ = s.blacklistTokenClaims(ctx, claims)
		}
	}

	s.clearSessionCookies(w)
	return nil
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

func (s *AuthService) ResetPassword(ctx context.Context, req dto.ResetPasswordRequest, r *http.Request, w http.ResponseWriter) error {
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
	if err := s.userRepo.UpdatePassword(ctx, user.ID, hash); err != nil {
		return fmt.Errorf("update password: %w", err)
	}
	if err := s.redisClient.Del(ctx, s.resetOTPKey(req.Email)).Err(); err != nil {
		return fmt.Errorf("delete reset otp: %w", err)
	}

	_ = s.Logout(ctx, r, w)
	return nil
}

func (s *AuthService) ChangePassword(ctx context.Context, userID string, req dto.ChangePasswordRequest, r *http.Request, w http.ResponseWriter) error {
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
	if err := s.userRepo.UpdatePassword(ctx, userID, hash); err != nil {
		return fmt.Errorf("update password: %w", err)
	}

	_ = s.Logout(ctx, r, w)
	return nil
}

func (s *AuthService) Me(ctx context.Context, userID string) (*dto.MeResponse, error) {
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("find me: %w", err)
	}
	return &dto.MeResponse{
		ID:       user.ID,
		Fullname: user.Fullname,
		Email:    user.Email,
		RoleID:   user.RoleID,
	}, nil
}

func (s *AuthService) AccessCookieName() string {
	return s.jwtCfg.AccessCookieName
}

func (s *AuthService) extractAccessToken(r *http.Request) string {
	if token := utils.GetCookieValue(r, s.jwtCfg.AccessCookieName); token != "" {
		return token
	}

	authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
	if authHeader == "" {
		return ""
	}
	const bearer = "Bearer "
	if !strings.HasPrefix(authHeader, bearer) {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(authHeader, bearer))
}

func (s *AuthService) issueSessionCookies(w http.ResponseWriter, userID string) error {
	accessToken, accessClaims, err := s.jwtService.GenerateAccessToken(userID)
	if err != nil {
		return fmt.Errorf("generate access token: %w", err)
	}
	refreshToken, refreshClaims, err := s.jwtService.GenerateRefreshToken(userID)
	if err != nil {
		return fmt.Errorf("generate refresh token: %w", err)
	}

	utils.SetTokenCookie(w, s.jwtCfg.AccessCookieName, accessToken, s.jwtCfg.AccessTTL, s.cookieConfig())
	utils.SetTokenCookie(w, s.jwtCfg.RefreshCookieName, refreshToken, s.jwtCfg.RefreshTTL, s.cookieConfig())

	if err := s.removeBlacklist(ctxWithoutCancel(), domain.TokenTypeAccess, accessClaims.ID); err != nil {
		return err
	}
	if err := s.removeBlacklist(ctxWithoutCancel(), domain.TokenTypeRefresh, refreshClaims.ID); err != nil {
		return err
	}

	return nil
}

func (s *AuthService) clearSessionCookies(w http.ResponseWriter) {
	utils.ClearTokenCookie(w, s.jwtCfg.AccessCookieName, s.cookieConfig())
	utils.ClearTokenCookie(w, s.jwtCfg.RefreshCookieName, s.cookieConfig())
}

func (s *AuthService) cookieConfig() utils.CookieConfig {
	return utils.CookieConfig{
		Secure:   s.cfg.CookieSecure,
		SameSite: s.cfg.CookieSameSite,
		Domain:   s.cfg.CookieDomain,
	}
}

func (s *AuthService) registerOTPKey(email string) string {
	return "auth:otp:register:" + email
}

func (s *AuthService) resetOTPKey(email string) string {
	return "auth:otp:reset:" + email
}

func (s *AuthService) blacklistKey(tokenType, jti string) string {
	return "auth:blacklist:" + tokenType + ":" + jti
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

func (s *AuthService) ensureNotBlacklisted(ctx context.Context, tokenType, jti string) error {
	result, err := s.redisClient.Exists(ctx, s.blacklistKey(tokenType, jti)).Result()
	if err != nil {
		return fmt.Errorf("check blacklist: %w", err)
	}
	if result > 0 {
		return apperrors.ErrTokenBlacklisted
	}
	return nil
}

func (s *AuthService) blacklistTokenClaims(ctx context.Context, claims *domain.TokenClaims) error {
	jti, err := TokenJTI(claims)
	if err != nil {
		return fmt.Errorf("extract token id: %w", err)
	}
	ttl := s.jwtService.RemainingTTL(claims)
	if ttl <= 0 {
		return nil
	}
	if err := s.redisClient.Set(ctx, s.blacklistKey(claims.TokenType, jti), "revoked", ttl).Err(); err != nil {
		return fmt.Errorf("blacklist token: %w", err)
	}
	return nil
}

func (s *AuthService) removeBlacklist(ctx context.Context, tokenType, jti string) error {
	if err := s.redisClient.Del(ctx, s.blacklistKey(tokenType, jti)).Err(); err != nil {
		return fmt.Errorf("remove blacklist key: %w", err)
	}
	return nil
}

func ctxWithoutCancel() context.Context {
	return context.Background()
}

