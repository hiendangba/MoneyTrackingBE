package httptransport

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/hiendangba/MoneyTrackingBE/Backend/api-gateway/internal/clients"
	"github.com/hiendangba/MoneyTrackingBE/Backend/api-gateway/internal/config"
	authv1 "github.com/hiendangba/MoneyTrackingBE/Backend/auth-service/gen/auth/v1"
	groupv1 "github.com/hiendangba/MoneyTrackingBE/Backend/group-service/gen/group/v1"
	transactionv1 "github.com/hiendangba/MoneyTrackingBE/Backend/transaction-service/gen/transaction/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type Gateway struct {
	cfg               config.Config
	clients           *clients.Clients
	authClient        authv1.AuthServiceClient
	groupClient       groupv1.GroupServiceClient
	transactionClient transactionv1.TransactionServiceClient
	logger            *slog.Logger
	originPattern     *regexp.Regexp
}

func NewGateway(cfg config.Config, c *clients.Clients, logger *slog.Logger) (*Gateway, error) {
	pattern, err := regexp.Compile(cfg.AllowedOriginRegex)
	if err != nil {
		return nil, fmt.Errorf("compile origin regex: %w", err)
	}
	return &Gateway{
		cfg:               cfg,
		clients:           c,
		authClient:        c.Auth,
		groupClient:       c.Group,
		transactionClient: c.Transaction,
		logger:            logger,
		originPattern:     pattern,
	}, nil
}

func (g *Gateway) Close() error {
	if g.clients == nil {
		return nil
	}
	return g.clients.Close()
}

func (g *Gateway) upstreamContext(ctx context.Context, r *http.Request, claims *AuthClaims) context.Context {
	md := metadata.MD{}
	if requestID, ok := ctx.Value(requestIDKey).(string); ok && requestID != "" {
		md.Set("x-request-id", requestID)
	}
	if traceparent := strings.TrimSpace(r.Header.Get("traceparent")); traceparent != "" {
		md.Set("traceparent", traceparent)
	}
	if tracestate := strings.TrimSpace(r.Header.Get("tracestate")); tracestate != "" {
		md.Set("tracestate", tracestate)
	}
	if claims != nil {
		md.Set("x-user-id", claims.UserID)
		md.Set("x-user-role", claims.RoleCode)
		md.Set("x-auth-token-type", "access")
		md.Set("x-auth-sub", claims.UserID)
		md.Set("x-auth-session-id", claims.SessionID)
		md.Set("x-auth-session-version", fmt.Sprintf("%d", claims.SessionVersion))
	}
	return metadata.NewOutgoingContext(ctx, md)
}

func (g *Gateway) decodeJSON(w http.ResponseWriter, r *http.Request, target any) error {
	defer func() { _ = r.Body.Close() }()

	contentType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || contentType != "application/json" {
		return fmt.Errorf("content type must be application/json")
	}

	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return errors.New("request body must contain one JSON object")
	}
	return nil
}

func writeJSON(w http.ResponseWriter, statusCode int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, err error, logger *slog.Logger) {
	code := codes.Internal
	statusCode := http.StatusInternalServerError
	message := "internal server error"

	if st, ok := status.FromError(err); ok {
		code = st.Code()
		statusCode = grpcStatusToHTTP(code)
		message = st.Message()
	} else if err != nil {
		message = err.Error()
	}

	if statusCode < 500 {
		message = err.Error()
	}
	if logger != nil && statusCode >= 500 {
		logger.Error("request failed", "status", statusCode, "error", err)
	}
	writeJSON(w, statusCode, ErrorResponse{
		Status:  statusCode,
		Code:    int(code),
		Message: message,
	})
}

func grpcStatusToHTTP(code codes.Code) int {
	switch code {
	case codes.InvalidArgument:
		return http.StatusBadRequest
	case codes.Unauthenticated:
		return http.StatusUnauthorized
	case codes.PermissionDenied:
		return http.StatusForbidden
	case codes.NotFound:
		return http.StatusNotFound
	case codes.AlreadyExists:
		return http.StatusConflict
	case codes.FailedPrecondition:
		return http.StatusBadRequest
	case codes.ResourceExhausted:
		return http.StatusTooManyRequests
	case codes.Unavailable:
		return http.StatusServiceUnavailable
	case codes.DeadlineExceeded:
		return http.StatusGatewayTimeout
	default:
		return http.StatusInternalServerError
	}
}

func requestContext(r *http.Request, timeout time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(r.Context(), timeout)
}
