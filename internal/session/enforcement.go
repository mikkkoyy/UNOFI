package session

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/unofi/unofi/internal/bandwidth"
	"github.com/unofi/unofi/internal/device"
	"github.com/unofi/unofi/internal/events"
	"github.com/unofi/unofi/internal/logger"
	"github.com/unofi/unofi/internal/mikrotik"
)

// EnforcementWorker continuously evaluates active sessions, synchronizes
// MikroTik usage, authorizes valid customers, and disconnects customers
// whose sessions are expired, exhausted, or cancelled.
type EnforcementWorker struct {
	mu             sync.Mutex
	router         mikrotik.Router
	sessionRepo    SessionRepository
	deviceRepo     DeviceGetter
	bandwidthMgr   *bandwidth.Manager
	eventBus       *events.Bus
	logger         *logger.Logger
	interval       time.Duration
	running        bool
	cancelFunc     context.CancelFunc
}

// DeviceGetter defines the minimal device lookup interface needed by the worker.
type DeviceGetter interface {
	GetByID(id int64) (*device.Device, error)
	GetByMAC(mac string) (*device.Device, error)
}

// NewEnforcementWorker creates a new session enforcement worker.
func NewEnforcementWorker(
	router mikrotik.Router,
	sessionRepo SessionRepository,
	deviceRepo DeviceGetter,
	bandwidthMgr *bandwidth.Manager,
	eventBus *events.Bus,
	loggr *logger.Logger,
	interval time.Duration,
) *EnforcementWorker {
	return &EnforcementWorker{
		router:       router,
		sessionRepo:  sessionRepo,
		deviceRepo:   deviceRepo,
		bandwidthMgr: bandwidthMgr,
		eventBus:     eventBus,
		logger:       loggr,
		interval:     interval,
	}
}

// Start begins the enforcement worker loop.
func (w *EnforcementWorker) Start(ctx context.Context) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.running {
		return fmt.Errorf("enforcement worker already running")
	}

	ctx, cancel := context.WithCancel(ctx)
	w.cancelFunc = cancel
	w.running = true

	w.logger.Info("session enforcement worker started (interval=%s)", w.interval)

	go w.run(ctx)

	return nil
}

// Stop gracefully stops the enforcement worker.
func (w *EnforcementWorker) Stop() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if !w.running {
		return
	}

	if w.cancelFunc != nil {
		w.cancelFunc()
	}
	w.running = false
	w.logger.Info("session enforcement worker stopped")
}

// IsRunning returns true if the worker is currently running.
func (w *EnforcementWorker) IsRunning() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.running
}

// run is the main worker loop.
func (w *EnforcementWorker) run(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	// Run immediately on start
	w.enforce(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.enforce(ctx)
		}
	}
}

// enforce performs a single enforcement cycle.
func (w *EnforcementWorker) enforce(ctx context.Context) {
	w.logger.Debug("session enforcement cycle starting")

	// Load all active sessions
	sessions, err := w.sessionRepo.ListActive()
	if err != nil {
		w.logger.Error("list active sessions: %v", err)
		return
	}

	if len(sessions) == 0 {
		w.logger.Debug("no active sessions to enforce")
		return
	}

	w.logger.Debug("enforcing %d active sessions", len(sessions))

	for _, sess := range sessions {
		// Check for context cancellation between sessions
		select {
		case <-ctx.Done():
			return
		default:
		}

		w.enforceSession(ctx, sess)
	}

	w.logger.Debug("session enforcement cycle complete")
}

// enforceSession enforces a single session.
func (w *EnforcementWorker) enforceSession(ctx context.Context, sess *Session) {
	// Find the associated device
	dev, err := w.deviceRepo.GetByID(sess.DeviceID)
	if err != nil {
		w.logger.Error("get device %d: %v", sess.DeviceID, err)
		return
	}

	if dev == nil {
		w.logger.Debug("session %d: device %d not found, skipping", sess.ID, sess.DeviceID)
		return
	}

	// Synchronize usage from MikroTik
	w.syncUsage(ctx, sess, dev)

	// Re-read session to get updated usage
	updatedSess, err := w.sessionRepo.GetByID(sess.ID)
	if err != nil {
		w.logger.Error("re-read session %d: %v", sess.ID, err)
		return
	}
	if updatedSess == nil {
		return
	}

	// Evaluate session validity
	if updatedSess.IsValid() {
		w.authorizeSession(ctx, updatedSess, dev)
	} else {
		w.deauthorizeSession(ctx, updatedSess, dev)
	}
}

// syncUsage synchronizes MikroTik usage data into the session.
func (w *EnforcementWorker) syncUsage(ctx context.Context, sess *Session, dev *device.Device) {
	if dev.MAC == "" {
		return
	}

	bytesIn, bytesOut, err := w.router.GetClientUsage(ctx, dev.MAC)
	if err != nil {
		// Router failure or client not found - don't change DB state
		w.logger.Debug("get client usage for %s: %v", dev.MAC, err)
		return
	}

	// Convert bytes to MB
	totalBytes := bytesIn + bytesOut
	mbUsed := float64(totalBytes) / (1024 * 1024)

	// Only update if usage increased (protect against counter resets)
	if mbUsed > sess.MBUsed {
		if err := w.sessionRepo.UpdateUsage(sess.ID, mbUsed, sess.TimeUsedSeconds); err != nil {
			w.logger.Error("update usage for session %d: %v", sess.ID, err)
		}
	}
}

// authorizeSession authorizes a valid session on MikroTik.
func (w *EnforcementWorker) authorizeSession(ctx context.Context, sess *Session, dev *device.Device) {
	if dev.MAC == "" || dev.IP == "" {
		return
	}

	// Authorize client on MikroTik (idempotent)
	if err := w.router.AuthorizeClient(ctx, dev.MAC, dev.IP); err != nil {
		w.logger.Error("authorize client mac=%s ip=%s: %v", dev.MAC, dev.IP, err)
		return
	}

	w.logger.Debug("session %d: authorized mac=%s ip=%s", sess.ID, dev.MAC, dev.IP)

	// Publish event
	if w.eventBus != nil {
		w.eventBus.Publish(events.Event{
			Type: events.EventClientAuthorized,
			Data: map[string]interface{}{
				"session_id": sess.ID,
				"device_id":  sess.DeviceID,
				"mac":        dev.MAC,
				"ip":         dev.IP,
			},
		})
	}
}

// deauthorizeSession deauthorizes an invalid session on MikroTik.
func (w *EnforcementWorker) deauthorizeSession(ctx context.Context, sess *Session, dev *device.Device) {
	if dev.MAC == "" {
		return
	}

	// Determine the reason for deauthorization
	reason := "expired"
	switch {
	case sess.Status == SessionCancelled:
		reason = "cancelled"
	case sess.IsExhausted():
		reason = "exhausted"
	case sess.IsExpired():
		reason = "expired"
	}

	// Update session status if still active
	if sess.Status == SessionActive {
		newStatus := SessionExpired
		if reason == "exhausted" {
			newStatus = SessionExhausted
		} else if reason == "cancelled" {
			newStatus = SessionCancelled
		}

		if err := w.sessionRepo.UpdateStatus(sess.ID, newStatus); err != nil {
			w.logger.Error("update session %d status to %s: %v", sess.ID, newStatus, err)
			return
		}

		w.logger.Info("session %d: %s (mac=%s, mb_limit=%.1f, mb_used=%.1f)",
			sess.ID, reason, dev.MAC, sess.MBLimit, sess.MBUsed)
	}

	// Deauthorize client on MikroTik (idempotent)
	if err := w.router.DeauthorizeClient(ctx, dev.MAC); err != nil {
		w.logger.Debug("deauthorize client mac=%s: %v", dev.MAC, err)
		// Don't return - the session status is already updated in DB
	}

	// Publish event
	if w.eventBus != nil {
		w.eventBus.Publish(events.Event{
			Type: events.EventClientDeauthorized,
			Data: map[string]interface{}{
				"session_id": sess.ID,
				"device_id":  sess.DeviceID,
				"mac":        dev.MAC,
				"reason":     reason,
			},
		})
	}
}
