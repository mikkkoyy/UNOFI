package api

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	session "github.com/unofi/unofi/internal/session"
)

// contextKey is a type for context keys to avoid collisions.
type contextKey string

const sessionContextKey contextKey = "session"

// contextWithSession adds a session to the context.
func contextWithSession(ctx context.Context, session interface{}) context.Context {
	return context.WithValue(ctx, sessionContextKey, session)
}

// sessionFromContext retrieves a session from the context.
func sessionFromContext(ctx context.Context) interface{} {
	return ctx.Value(sessionContextKey)
}

// handleStatus returns basic system status (public).
func (r *Router) handleStatus(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"version": "0.1.0",
	})
}

// handleListPackages returns available packages (public).
func (r *Router) handleListPackages(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	packages, err := r.container.PricingEngine.ListPackages(true)
	if err != nil {
		r.logger.Error("failed to list packages: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to list packages")
		return
	}

	writeJSON(w, http.StatusOK, packages)
}

// handlePortalInfo returns basic portal information and available capabilities.
func (r *Router) handlePortalInfo(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"version": "0.1.0",
		"capabilities": "packages, session management",
	})
}

// handleSessionStatus returns the current session for the identified device.
func (r *Router) handleSessionStatus(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Identify device from request context
	deviceID := r.identifyDeviceFromRequest(req)
	if deviceID == 0 {
		writeError(w, http.StatusUnauthorized, "device not identified")
		return
	}

	sess, err := r.container.SessionRepo.GetActiveByDevice(deviceID)
	if err != nil {
		r.logger.Error("failed to get session: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to retrieve session")
		return
	}

	if sess == nil {
		writeJSON(w, http.StatusOK, map[string]string{
			"session_status": "no_active_session",
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"session_status":   sess.Status,
		"package_id":       sess.VoucherID,
		"start_time":       sess.CreatedAt,
		"expiration_time":  sess.ExpiresAt,
		"remaining_time":   sess.RemainingTime(),
		"mb_limit":         sess.MBLimit,
		"mb_used":          sess.MBUsed,
		"remaining_mb":     sess.RemainingMB(),
		"device_state":     "online",
		"connection_state": "authorized",
	})
}

// identifyDeviceFromRequest identifies the device from the request.
// It looks up the device by MAC address from query parameters or context.
// The MAC should be provided by the MikroTik HotSpot redirect context.
func (r *Router) identifyDeviceFromRequest(req *http.Request) int64 {
	mac := req.URL.Query().Get("mac")
	ip := req.URL.Query().Get("ip")

	if mac == "" && ip == "" {
		return 0
	}

	// Try to find device by MAC or IP
	device, err := r.container.DeviceRepo.GetByMAC(mac)
	if err != nil {
		r.logger.Error("failed to look up device by MAC: %v", err)
		return 0
	}

	if device == nil {
		return 0
	}

	return device.ID
}

// handleSessionConnect starts a customer session for the identified device.
func (r *Router) handleSessionConnect(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Identify device from request
	deviceID := r.identifyDeviceFromRequest(req)
	if deviceID == 0 {
		writeError(w, http.StatusBadRequest, "device not identified - provide mac or ip query parameter")
		return
	}

	// Check for existing active session
	existing, err := r.container.SessionRepo.GetActiveByDevice(deviceID)
	if err != nil {
		r.logger.Error("failed to check existing session: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	if existing != nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"session_id":       existing.ID,
			"session_status":   existing.Status,
			"message": "session already active",
		})
		return
	}

	// Read package ID from request body
	var connectReq struct {
		PackageID int64 `json:"package_id"`
	}
	if err := json.NewDecoder(req.Body).Decode(&connectReq); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body, package_id required")
		return
	}

	// Validate package exists and is active
	pkg, err := r.container.PricingEngine.GetPackage(connectReq.PackageID)
	if err != nil {
		r.logger.Error("failed to get package: %v", err)
		writeError(w, http.StatusBadRequest, "invalid or unavailable package")
		return
	}

	if !pkg.IsActive {
		writeError(w, http.StatusBadRequest, "package is not active")
		return
	}

	// Calculate grant (MB and time)
	mbGranted, timeSeconds, err := r.container.PricingEngine.CalculateGrant(connectReq.PackageID)
	if err != nil {
		r.logger.Error("failed to calculate grant: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	// Create a new session
	newSess := &session.Session{
		DeviceID:        deviceID,
		MBLimit:         mbGranted,
		TimeLimitSeconds: timeSeconds,
		MBUsed:          0,
		TimeUsedSeconds: 0,
		Status:          session.SessionActive,
		ExpiresAt:       time.Now().Add(time.Second * time.Duration(timeSeconds)),
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}

	createdSession, err := r.container.SessionRepo.Create(newSess)
	if err != nil {
		r.logger.Error("failed to create session: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to create session")
		return
	}

	// Authorize the client on MikroTik
	if r.container.Router != nil {
		mac := req.URL.Query().Get("mac")
		ip := req.URL.Query().Get("ip")
		if mac != "" && ip != "" {
			authErr := r.container.Router.AuthorizeClient(nil, mac, ip)
			if authErr != nil {
				r.logger.Warn("failed to authorize client on MikroTik: %v", authErr)
				// Don't fail the API call - enforcement worker will handle this
			}
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"session_id":       createdSession.ID,
		"session_status":   createdSession.Status,
		"mb_limit":         createdSession.MBLimit,
		"mb_used":          createdSession.MBUsed,
		"remaining_mb":     createdSession.RemainingMB(),
		"time_limit_seconds": createdSession.TimeLimitSeconds,
		"time_used_seconds": createdSession.TimeUsedSeconds,
		"remaining_time":   createdSession.RemainingTime(),
		"package_id":       connectReq.PackageID,
		"message": "session started",
	})
}

// handleSessionDisconnect ends the customer's session.
func (r *Router) handleSessionDisconnect(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Identify device from request
	deviceID := r.identifyDeviceFromRequest(req)
	if deviceID == 0 {
		writeError(w, http.StatusBadRequest, "device not identified - provide mac or ip query parameter")
		return
	}

	// Find the active session for this device
	sess, err := r.container.SessionRepo.GetActiveByDevice(deviceID)
	if err != nil {
		r.logger.Error("failed to find session: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	if sess == nil {
		writeJSON(w, http.StatusOK, map[string]string{
			"message": "no active session to cancel",
		})
		return
	}

	// Cancel the session
	cancelErr := r.container.SessionRepo.UpdateStatus(sess.ID, session.SessionCancelled)
	if cancelErr != nil {
		r.logger.Error("failed to cancel session: %v", cancelErr)
		writeError(w, http.StatusInternalServerError, "failed to cancel session")
		return
	}

	// Deauthorize the client on MikroTik
	mac := req.URL.Query().Get("mac")
	ip := req.URL.Query().Get("ip")
	if mac != "" && ip != "" && r.container.Router != nil {
		deauthErr := r.container.Router.DeauthorizeClient(nil, mac)
		if deauthErr != nil {
			r.logger.Warn("failed to deauthorize client on MikroTik: %v", deauthErr)
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"session_id":    sess.ID,
		"session_status": sess.Status,
		"message": "session cancelled",
	})
}
