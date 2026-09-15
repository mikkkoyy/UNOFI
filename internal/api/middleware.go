package api

import (
	"encoding/json"
	"net/http"
	"strings"
)

// writeJSON writes a JSON response.
func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// writeError writes a JSON error response.
func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

// getAuthToken extracts the auth token from the request.
// Checks cookie first, then Authorization header.
func getAuthToken(r *http.Request) string {
	// Check cookie
	c, err := r.Cookie("unofi_session")
	if err == nil && c.Value != "" {
		return c.Value
	}

	// Check Authorization header
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}

	return ""
}

// setSessionCookie sets the session cookie.
func setSessionCookie(w http.ResponseWriter, token string, maxAge int) {
	http.SetCookie(w, &http.Cookie{
		Name:     "unofi_session",
		Value:    token,
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		Secure:   false, // Set to true in production with HTTPS
	})
}

// clearSessionCookie removes the session cookie.
func clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:   "unofi_session",
		Value:  "",
		Path:   "/",
		MaxAge: -1,
	})
}

// requireAuth wraps a handler to require authentication.
func (r *Router) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		token := getAuthToken(req)
		if token == "" {
			writeError(w, http.StatusUnauthorized, "authentication required")
			return
		}

		session := r.container.AuthStore.ValidateSession(token)
		if session == nil {
			writeError(w, http.StatusUnauthorized, "session expired or invalid")
			return
		}

		// Store session in request context for handlers
		ctx := contextWithSession(req.Context(), session)
		next(w, req.WithContext(ctx))
	}
}
