package httptransport

import (
	"log/slog"
	"net/http"
)

func recoveryMiddleware(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				if logger != nil {
					logger.Error("panic recovered", "panic", rec)
				}
				writeJSON(w, http.StatusInternalServerError, map[string]string{"message": "internal server error"})
			}
		}()
		next.ServeHTTP(w, r)
	})
}
