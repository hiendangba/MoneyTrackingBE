package httptransport

import (
	"net/http"
	"time"

	authv1 "auth-service/gen/auth/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (g *Gateway) Health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, MessageResponse{Message: "ok"})
}

func (g *Gateway) JWKS(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := requestContext(r, g.cfg.RequestTimeout)
	defer cancel()

	resp, err := g.authClient.GetJWKS(g.upstreamContext(ctx, r, nil), &authv1.GetJWKSRequest{})
	if err != nil {
		writeError(w, err, g.logger)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=300, stale-while-revalidate=60")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(resp.GetJwksJson()))
}

func (g *Gateway) CSRF(w http.ResponseWriter, _ *http.Request) {
	token := randomToken(32)
	http.SetCookie(w, &http.Cookie{
		Name:     g.cfg.CSRF.CookieName,
		Value:    token,
		Path:     "/",
		Domain:   g.cfg.Cookie.Domain,
		Expires:  time.Now().Add(30 * time.Minute),
		MaxAge:   maxAgeFromTTL(30 * time.Minute),
		Secure:   g.cfg.Cookie.Secure,
		HttpOnly: false,
		SameSite: sameSiteFromString(g.cfg.Cookie.SameSite),
	})
	writeJSON(w, http.StatusOK, CSRFResponse{CSRFToken: token})
}

func (g *Gateway) Register(w http.ResponseWriter, r *http.Request) {
	var req RegisterRequest
	if err := g.decodeJSON(w, r, &req); err != nil {
		writeError(w, err, g.logger)
		return
	}
	ctx, cancel := requestContext(r, g.cfg.RequestTimeout)
	defer cancel()
	_, err := g.authClient.Register(g.upstreamContext(ctx, r, nil), &authv1.RegisterRequest{
		Fullname: req.Fullname,
		Email:    req.Email,
		Password: req.Password,
	})
	if err != nil {
		writeError(w, err, g.logger)
		return
	}
	writeJSON(w, http.StatusCreated, MessageResponse{Message: "otp sent"})
}

func (g *Gateway) VerifyOTP(w http.ResponseWriter, r *http.Request) {
	var req VerifyOTPRequest
	if err := g.decodeJSON(w, r, &req); err != nil {
		writeError(w, err, g.logger)
		return
	}
	ctx, cancel := requestContext(r, g.cfg.RequestTimeout)
	defer cancel()
	resp, err := g.authClient.VerifyOTP(g.upstreamContext(ctx, r, nil), &authv1.VerifyOTPRequest{
		Email:   req.Email,
		Otp:     req.OTP,
		Purpose: req.Purpose,
	})
	if err != nil {
		writeError(w, err, g.logger)
		return
	}
	g.setBrowserSessionCookies(w, resp)
	writeJSON(w, http.StatusOK, MessageResponse{Message: "otp verified"})
}

func (g *Gateway) Login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := g.decodeJSON(w, r, &req); err != nil {
		writeError(w, err, g.logger)
		return
	}
	ctx, cancel := requestContext(r, g.cfg.RequestTimeout)
	defer cancel()
	resp, err := g.authClient.Login(g.upstreamContext(ctx, r, nil), &authv1.LoginRequest{
		Email:    req.Email,
		Password: req.Password,
	})
	if err != nil {
		writeError(w, err, g.logger)
		return
	}
	g.setBrowserSessionCookies(w, resp)
	writeJSON(w, http.StatusOK, MessageResponse{Message: "logged in"})
}

func (g *Gateway) RefreshToken(w http.ResponseWriter, r *http.Request) {
	refreshToken := g.refreshTokenFromRequest(r)
	if refreshToken == "" {
		writeError(w, status.Error(codes.Unauthenticated, "missing refresh token"), g.logger)
		return
	}
	ctx, cancel := requestContext(r, g.cfg.RequestTimeout)
	defer cancel()
	resp, err := g.authClient.RefreshToken(g.upstreamContext(ctx, r, nil), &authv1.RefreshTokenRequest{
		RefreshToken: refreshToken,
	})
	if err != nil {
		writeError(w, err, g.logger)
		return
	}
	g.setBrowserSessionCookies(w, resp)
	writeJSON(w, http.StatusOK, MessageResponse{Message: "token refreshed"})
}

func (g *Gateway) Logout(w http.ResponseWriter, r *http.Request) {
	defer g.clearBrowserSessionCookies(w)
	ctx, cancel := requestContext(r, g.cfg.RequestTimeout)
	defer cancel()
	_, err := g.authClient.Logout(g.upstreamContext(ctx, r, nil), &authv1.LogoutRequest{
		AccessToken:  extractToken(r),
		RefreshToken: g.refreshTokenFromRequest(r),
	})
	if err != nil {
		writeError(w, err, g.logger)
		return
	}
	writeJSON(w, http.StatusOK, MessageResponse{Message: "logged out"})
}

func (g *Gateway) ForgotPassword(w http.ResponseWriter, r *http.Request) {
	var req ForgotPasswordRequest
	if err := g.decodeJSON(w, r, &req); err != nil {
		writeError(w, err, g.logger)
		return
	}
	ctx, cancel := requestContext(r, g.cfg.RequestTimeout)
	defer cancel()
	_, err := g.authClient.ForgotPassword(g.upstreamContext(ctx, r, nil), &authv1.ForgotPasswordRequest{Email: req.Email})
	if err != nil {
		writeError(w, err, g.logger)
		return
	}
	writeJSON(w, http.StatusOK, MessageResponse{Message: "if the email exists, an otp has been sent"})
}

func (g *Gateway) ResetPassword(w http.ResponseWriter, r *http.Request) {
	var req ResetPasswordRequest
	if err := g.decodeJSON(w, r, &req); err != nil {
		writeError(w, err, g.logger)
		return
	}
	ctx, cancel := requestContext(r, g.cfg.RequestTimeout)
	defer cancel()
	_, err := g.authClient.ResetPassword(g.upstreamContext(ctx, r, nil), &authv1.ResetPasswordRequest{
		Email:       req.Email,
		Otp:         req.OTP,
		NewPassword: req.NewPassword,
	})
	if err != nil {
		writeError(w, err, g.logger)
		return
	}
	g.clearBrowserSessionCookies(w)
	writeJSON(w, http.StatusOK, MessageResponse{Message: "password reset successfully"})
}

func (g *Gateway) ChangePassword(w http.ResponseWriter, r *http.Request, claims AuthClaims) {
	var req ChangePasswordRequest
	if err := g.decodeJSON(w, r, &req); err != nil {
		writeError(w, err, g.logger)
		return
	}
	ctx, cancel := requestContext(r, g.cfg.RequestTimeout)
	defer cancel()
	_, err := g.authClient.ChangePassword(g.upstreamContext(ctx, r, &claims), &authv1.ChangePasswordRequest{
		UserId:          claims.UserID,
		CurrentPassword: req.CurrentPassword,
		NewPassword:     req.NewPassword,
	})
	if err != nil {
		writeError(w, err, g.logger)
		return
	}
	g.clearBrowserSessionCookies(w)
	writeJSON(w, http.StatusOK, MessageResponse{Message: "password changed successfully"})
}

func (g *Gateway) Me(w http.ResponseWriter, r *http.Request, claims AuthClaims) {
	ctx, cancel := requestContext(r, g.cfg.RequestTimeout)
	defer cancel()
	resp, err := g.authClient.Me(g.upstreamContext(ctx, r, &claims), &authv1.MeRequest{UserId: claims.UserID})
	if err != nil {
		writeError(w, err, g.logger)
		return
	}
	writeJSON(w, http.StatusOK, MeResponse{
		ID:       resp.GetId(),
		Fullname: resp.GetFullname(),
		Email:    resp.GetEmail(),
		RoleID:   resp.GetRoleId(),
		RoleCode: resp.GetRoleCode(),
	})
}

func (g *Gateway) MobileLogin(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := g.decodeJSON(w, r, &req); err != nil {
		writeError(w, err, g.logger)
		return
	}
	ctx, cancel := requestContext(r, g.cfg.RequestTimeout)
	defer cancel()
	resp, err := g.authClient.Login(g.upstreamContext(ctx, r, nil), &authv1.LoginRequest{
		Email:    req.Email,
		Password: req.Password,
	})
	if err != nil {
		writeError(w, err, g.logger)
		return
	}
	writeJSON(w, http.StatusOK, toTokenResponse(resp))
}

func (g *Gateway) MobileRefresh(w http.ResponseWriter, r *http.Request) {
	var req RefreshRequest
	if err := g.decodeJSON(w, r, &req); err != nil {
		writeError(w, err, g.logger)
		return
	}
	ctx, cancel := requestContext(r, g.cfg.RequestTimeout)
	defer cancel()
	resp, err := g.authClient.RefreshToken(g.upstreamContext(ctx, r, nil), &authv1.RefreshTokenRequest{
		RefreshToken: req.RefreshToken,
	})
	if err != nil {
		writeError(w, err, g.logger)
		return
	}
	writeJSON(w, http.StatusOK, toTokenResponse(resp))
}

func (g *Gateway) MobileLogout(w http.ResponseWriter, r *http.Request) {
	var req LogoutRequest
	if err := g.decodeJSON(w, r, &req); err != nil {
		writeError(w, err, g.logger)
		return
	}
	ctx, cancel := requestContext(r, g.cfg.RequestTimeout)
	defer cancel()
	_, err := g.authClient.Logout(g.upstreamContext(ctx, r, nil), &authv1.LogoutRequest{
		AccessToken:  extractToken(r),
		RefreshToken: req.RefreshToken,
	})
	if err != nil {
		writeError(w, err, g.logger)
		return
	}
	writeJSON(w, http.StatusOK, MessageResponse{Message: "logged out"})
}

func (g *Gateway) setBrowserSessionCookies(w http.ResponseWriter, resp *authv1.TokenResponse) {
	http.SetCookie(w, &http.Cookie{
		Name:     "access_token",
		Value:    resp.GetAccessToken(),
		Path:     "/",
		Domain:   g.cfg.Cookie.Domain,
		Expires:  time.Now().Add(g.cfg.AccessTTL),
		MaxAge:   maxAgeFromTTL(g.cfg.AccessTTL),
		Secure:   g.cfg.Cookie.Secure,
		HttpOnly: true,
		SameSite: sameSiteFromString(g.cfg.Cookie.SameSite),
	})
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    resp.GetRefreshToken(),
		Path:     "/api/auth",
		Domain:   g.cfg.Cookie.Domain,
		Expires:  time.Now().Add(g.cfg.RefreshTTL),
		MaxAge:   maxAgeFromTTL(g.cfg.RefreshTTL),
		Secure:   g.cfg.Cookie.Secure,
		HttpOnly: true,
		SameSite: sameSiteFromString(g.cfg.Cookie.SameSite),
	})
}

func (g *Gateway) clearBrowserSessionCookies(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     "access_token",
		Value:    "",
		Path:     "/",
		Domain:   g.cfg.Cookie.Domain,
		MaxAge:   -1,
		Secure:   g.cfg.Cookie.Secure,
		HttpOnly: true,
		SameSite: sameSiteFromString(g.cfg.Cookie.SameSite),
	})
	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    "",
		Path:     "/api/auth",
		Domain:   g.cfg.Cookie.Domain,
		MaxAge:   -1,
		Secure:   g.cfg.Cookie.Secure,
		HttpOnly: true,
		SameSite: sameSiteFromString(g.cfg.Cookie.SameSite),
	})
}

func (g *Gateway) refreshTokenFromRequest(r *http.Request) string {
	if cookie, err := r.Cookie("refresh_token"); err == nil && cookie.Value != "" {
		return cookie.Value
	}
	return ""
}

func toTokenResponse(resp *authv1.TokenResponse) TokenResponse {
	if resp == nil {
		return TokenResponse{}
	}
	return TokenResponse{
		AccessToken:      resp.GetAccessToken(),
		RefreshToken:     resp.GetRefreshToken(),
		TokenType:        resp.GetTokenType(),
		ExpiresIn:        resp.GetExpiresIn(),
		RefreshExpiresIn: resp.GetRefreshExpiresIn(),
	}
}
