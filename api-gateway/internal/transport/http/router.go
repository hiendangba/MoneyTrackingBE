package httptransport

import "net/http"

func (g *Gateway) Router() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", g.Health)
	mux.HandleFunc("GET /.well-known/jwks.json", g.JWKS)
	mux.HandleFunc("GET /api/auth/csrf", g.CSRF)

	mux.Handle("POST /api/auth/register", g.requireCSRF(http.HandlerFunc(g.Register)))
	mux.Handle("POST /api/auth/verify-otp", g.requireCSRF(http.HandlerFunc(g.VerifyOTP)))
	mux.Handle("POST /api/auth/login", g.requireCSRF(http.HandlerFunc(g.Login)))
	mux.Handle("POST /api/auth/refresh-token", g.requireCSRF(http.HandlerFunc(g.RefreshToken)))
	mux.Handle("POST /api/auth/logout", g.requireCSRF(http.HandlerFunc(g.Logout)))
	mux.Handle("POST /api/auth/forgot-password", g.requireCSRF(http.HandlerFunc(g.ForgotPassword)))
	mux.Handle("POST /api/auth/reset-password", g.requireCSRF(http.HandlerFunc(g.ResetPassword)))

	mux.Handle("POST /api/auth/mobile/login", http.HandlerFunc(g.MobileLogin))
	mux.Handle("POST /api/auth/mobile/refresh", http.HandlerFunc(g.MobileRefresh))
	mux.Handle("POST /api/auth/mobile/logout", http.HandlerFunc(g.MobileLogout))

	protected := func(next func(http.ResponseWriter, *http.Request, AuthClaims)) http.Handler {
		return g.requireAuth(next)
	}
	admin := func(next func(http.ResponseWriter, *http.Request, AuthClaims)) http.Handler {
		return g.requireRole(g.cfg.Auth.AdminRoleCode, next)
	}

	mux.Handle("POST /api/auth/change-password", g.requireCSRF(protected(g.ChangePassword)))
	mux.Handle("GET /api/auth/me", protected(g.Me))

	mux.Handle("POST /api/menus", g.requireCSRF(admin(g.CreateMenu)))
	mux.Handle("GET /api/menus", admin(g.ListMenus))
	mux.Handle("GET /api/menus/tree", protected(g.TreeMenus))
	mux.Handle("GET /api/menus/{id}", admin(g.GetMenu))
	mux.Handle("PATCH /api/menus/{id}", g.requireCSRF(admin(g.UpdateMenu)))
	mux.Handle("DELETE /api/menus/{id}", g.requireCSRF(admin(g.DeleteMenu)))

	mux.Handle("GET /api/groups", protected(g.ListGroups))
	mux.Handle("POST /api/groups", g.requireCSRF(protected(g.CreateGroup)))
	mux.Handle("GET /api/groups/{id}", protected(g.GetGroup))
	mux.Handle("PATCH /api/groups/{id}", g.requireCSRF(protected(g.UpdateGroup)))
	mux.Handle("DELETE /api/groups/{id}", g.requireCSRF(protected(g.DeleteGroup)))

	mux.Handle("GET /api/groups/{id}/members", protected(g.ListMembers))
	mux.Handle("POST /api/groups/{id}/members", g.requireCSRF(protected(g.AddMember)))
	mux.Handle("DELETE /api/groups/{id}/members/{user_id}", g.requireCSRF(protected(g.RemoveMember)))

	mux.Handle("GET /api/groups/{id}/invitations", protected(g.ListInvitations))
	mux.Handle("POST /api/groups/{id}/invitations", g.requireCSRF(protected(g.InviteMember)))
	mux.Handle("POST /api/invitations/{id}/respond", g.requireCSRF(protected(g.RespondInvitation)))

	handler := requestIDMiddleware(securityHeaders(corsMiddleware(g.cfg.AllowedOriginRegex, mux)))
	return recoverMiddleware(g.logger, handler)
}
