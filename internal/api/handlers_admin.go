package api

import (
	"encoding/json"
	"net/http"

	"github.com/unofi/unofi/internal/auth"
)

// handleAdminLogin authenticates an admin user.
func (r *Router) handleAdminLogin(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var loginReq struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	if err := json.NewDecoder(req.Body).Decode(&loginReq); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if loginReq.Username == "" || loginReq.Password == "" {
		writeError(w, http.StatusBadRequest, "username and password required")
		return
	}

	// Check if this is the first login (no admin users exist)
	count, err := r.container.AuthRepo.Count()
	if err != nil {
		r.logger.Error("failed to count admin users: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	// First user: create account
	if count == 0 {
		if len(loginReq.Password) < 6 {
			writeError(w, http.StatusBadRequest, "password must be at least 6 characters")
			return
		}

		hash, err := auth.HashPassword(loginReq.Password)
		if err != nil {
			r.logger.Error("failed to hash password: %v", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}

		_, err = r.container.AuthRepo.Create(loginReq.Username, hash)
		if err != nil {
			r.logger.Error("failed to create admin: %v", err)
			writeError(w, http.StatusInternalServerError, "failed to create admin")
			return
		}

		// Create session
		sess, err := r.container.AuthStore.CreateSession(1, loginReq.Username)
		if err != nil {
			r.logger.Error("failed to create session: %v", err)
			writeError(w, http.StatusInternalServerError, "internal error")
			return
		}

		setSessionCookie(w, sess.Token, int(r.container.AuthStore.TTL().Seconds()))
		writeJSON(w, http.StatusOK, map[string]string{
			"status": "init",
			"token":  sess.Token,
		})
		return
	}

	// Existing user: verify credentials
	user, err := r.container.AuthRepo.GetByUsername(loginReq.Username)
	if err != nil || user == nil {
		writeError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	if !auth.VerifyPassword(loginReq.Password, user.PasswordHash) {
		writeError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	// Create session
	sess, err := r.container.AuthStore.CreateSession(user.ID, user.Username)
	if err != nil {
		r.logger.Error("failed to create session: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	setSessionCookie(w, sess.Token, int(r.container.AuthStore.TTL().Seconds()))
	writeJSON(w, http.StatusOK, map[string]string{
		"status": "ok",
		"token":  sess.Token,
	})
}

// handleAdminLogout destroys the current session.
func (r *Router) handleAdminLogout(w http.ResponseWriter, req *http.Request) {
	token := getAuthToken(req)
	if token != "" {
		r.container.AuthStore.DestroySession(token)
	}
	clearSessionCookie(w)
	writeJSON(w, http.StatusOK, map[string]string{"status": "logged_out"})
}

// handleAdminCheck verifies the current session.
func (r *Router) handleAdminCheck(w http.ResponseWriter, req *http.Request) {
	token := getAuthToken(req)
	session := r.container.AuthStore.ValidateSession(token)
	if session == nil {
		writeError(w, http.StatusUnauthorized, "session expired")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"status":   "ok",
		"username": session.Username,
	})
}

// handleAdminDevices returns the device list.
func (r *Router) handleAdminDevices(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	devices, err := r.container.DeviceRepo.List(50, 0)
	if err != nil {
		r.logger.Error("failed to list devices: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to list devices")
		return
	}

	writeJSON(w, http.StatusOK, devices)
}

// handleAdminSessions returns active sessions.
func (r *Router) handleAdminSessions(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	sessions, err := r.container.SessionRepo.ListActive()
	if err != nil {
		r.logger.Error("failed to list sessions: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to list sessions")
		return
	}

	writeJSON(w, http.StatusOK, sessions)
}

// handleAdminTransactions returns transaction history.
func (r *Router) handleAdminTransactions(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	transactions, err := r.container.TransactionRepo.List(50, 0)
	if err != nil {
		r.logger.Error("failed to list transactions: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to list transactions")
		return
	}

	writeJSON(w, http.StatusOK, transactions)
}

// handleAdminRouter returns MikroTik system info.
func (r *Router) handleAdminRouter(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	info, err := r.container.Router.GetInfo(nil)
	if err != nil {
		r.logger.Error("failed to get router info: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to get router info")
		return
	}

	writeJSON(w, http.StatusOK, info)
}
