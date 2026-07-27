package service

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/hiendangba/MoneyTrackingBE/Backend/auth-service/internal/config"
	"github.com/hiendangba/MoneyTrackingBE/Backend/auth-service/internal/domain"
	apperrors "github.com/hiendangba/MoneyTrackingBE/Backend/auth-service/internal/errors"

	"github.com/alicebob/miniredis/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/redis/go-redis/v9"
)

func TestRefreshTokenIsSingleUseAndReplayRevokesFamily(t *testing.T) {
	t.Parallel()

	authService, jwtService, redisServer := newTestAuthService(t)
	defer redisServer.Close()

	refreshToken, _, err := jwtService.GenerateRefreshToken("user-1", "session-1", 1)
	if err != nil {
		t.Fatalf("GenerateRefreshToken() error = %v", err)
	}

	const requests = 2
	results := make(chan error, requests)
	responses := make(chan string, requests)
	var wait sync.WaitGroup
	for range requests {
		wait.Add(1)
		go func() {
			defer wait.Done()
			response, refreshErr := authService.MobileRefresh(context.Background(), refreshToken)
			if response != nil {
				responses <- response.RefreshToken
			}
			results <- refreshErr
		}()
	}
	wait.Wait()
	close(results)
	close(responses)

	successes := 0
	for refreshErr := range results {
		if refreshErr == nil {
			successes++
		}
	}
	if successes != 1 {
		t.Fatalf("concurrent refresh successes = %d, want 1", successes)
	}

	rotatedToken := <-responses
	if _, err := authService.MobileRefresh(context.Background(), rotatedToken); !errors.Is(err, apperrors.ErrSessionRevoked) {
		t.Fatalf("MobileRefresh() after replay error = %v, want ErrSessionRevoked", err)
	}
}

func TestLogoutFailsClosedWhenRedisIsUnavailable(t *testing.T) {
	t.Parallel()

	authService, jwtService, redisServer := newTestAuthService(t)
	accessToken, _, err := jwtService.GenerateAccessToken("user-1", "role-1", "member", "session-1", 1)
	if err != nil {
		t.Fatalf("GenerateAccessToken() error = %v", err)
	}
	redisServer.Close()

	if err := authService.MobileLogout(context.Background(), accessToken, ""); err == nil {
		t.Fatal("MobileLogout() succeeded while Redis was unavailable")
	}
}

func TestLogoutRevokesRefreshFamilyWhenAccessTokenIsExpired(t *testing.T) {
	t.Parallel()

	authService, jwtService, redisServer := newTestAuthService(t)
	defer redisServer.Close()

	refreshToken, _, err := jwtService.GenerateRefreshToken("user-1", "session-1", 1)
	if err != nil {
		t.Fatalf("GenerateRefreshToken() error = %v", err)
	}
	claims := validTestClaims(domain.TokenTypeAccess, "money-tracking-api")
	claims.SessionID = "session-1"
	claims.IssuedAt = jwt.NewNumericDate(time.Now().Add(-10 * time.Minute))
	claims.NotBefore = jwt.NewNumericDate(time.Now().Add(-10 * time.Minute))
	claims.ExpiresAt = jwt.NewNumericDate(time.Now().Add(-5 * time.Minute))
	expiredAccessToken := signTestToken(t, claims, jwtService.privateKey, jwtService.KeyID())
	if err := authService.MobileLogout(context.Background(), expiredAccessToken, refreshToken); err != nil {
		t.Fatalf("MobileLogout() error = %v", err)
	}
	if _, err := authService.MobileRefresh(context.Background(), refreshToken); !errors.Is(err, apperrors.ErrSessionRevoked) {
		t.Fatalf("MobileRefresh() after logout error = %v, want ErrSessionRevoked", err)
	}
}

func TestRefreshRejectsChangedSessionVersionAndInactiveRole(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		user domain.User
		want error
	}{
		{
			name: "changed session version",
			user: activeTestUser(2),
			want: apperrors.ErrSessionRevoked,
		},
		{
			name: "inactive role",
			user: func() domain.User {
				user := activeTestUser(1)
				user.RoleActive = false
				return user
			}(),
			want: apperrors.ErrAccountInactive,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			authService, jwtService, redisServer := newTestAuthServiceWithUser(t, tt.user)
			defer redisServer.Close()
			refreshToken, _, err := jwtService.GenerateRefreshToken("user-1", "session-1", 1)
			if err != nil {
				t.Fatalf("GenerateRefreshToken() error = %v", err)
			}
			if _, err := authService.MobileRefresh(context.Background(), refreshToken); !errors.Is(err, tt.want) {
				t.Fatalf("MobileRefresh() error = %v, want %v", err, tt.want)
			}
		})
	}
}

func newTestAuthService(t *testing.T) (*AuthService, *JWTService, *miniredis.Miniredis) {
	t.Helper()
	return newTestAuthServiceWithUser(t, activeTestUser(1))
}

func newTestAuthServiceWithUser(t *testing.T, user domain.User) (*AuthService, *JWTService, *miniredis.Miniredis) {
	t.Helper()
	jwtService, _, _ := newTestJWTService(t)
	redisServer := miniredis.RunT(t)
	redisClient := redis.NewClient(&redis.Options{Addr: redisServer.Addr()})
	t.Cleanup(func() {
		_ = redisClient.Close()
	})
	authService := NewAuthService(
		config.AuthConfig{},
		&stubUserRepository{user: user},
		jwtService,
		redisClient,
		stubPublisher{},
	)
	return authService, jwtService, redisServer
}

func activeTestUser(sessionVersion int64) domain.User {
	return domain.User{
		ID:             "user-1",
		Email:          "user@example.com",
		RoleID:         "role-1",
		RoleCode:       "member",
		RoleActive:     true,
		SessionVersion: sessionVersion,
		IsActive:       true,
	}
}

type stubUserRepository struct {
	user domain.User
}

func (r *stubUserRepository) FindByEmail(context.Context, string) (*domain.User, error) {
	user := r.user
	return &user, nil
}

func (r *stubUserRepository) FindByID(context.Context, string) (*domain.User, error) {
	user := r.user
	return &user, nil
}

func (r *stubUserRepository) FindRoleIDByCode(context.Context, string) (string, error) {
	return r.user.RoleID, nil
}

func (r *stubUserRepository) Create(_ context.Context, user domain.User) (*domain.User, error) {
	return &user, nil
}

func (r *stubUserRepository) UpdatePasswordAndIncrementSessionVersion(context.Context, string, string) (int64, error) {
	r.user.SessionVersion++
	return r.user.SessionVersion, nil
}

type stubPublisher struct{}

func (stubPublisher) PublishRegisterOTP(context.Context, string, string, string, time.Duration) error {
	return nil
}

func (stubPublisher) PublishResetOTP(context.Context, string, string, time.Duration) error {
	return nil
}

func (stubPublisher) Close() error {
	return nil
}
