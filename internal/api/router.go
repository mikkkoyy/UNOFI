package api

import (
	"net/http"
	"sync"
	"time"

	"github.com/unofi/unofi/internal/logger"
	"github.com/unofi/unofi/internal/monitoring"
	"github.com/unofi/unofi/internal/service"
)

// rateLimiter tracks request rates per key.
type rateLimiter struct {
	mu       sync.Mutex
	interval time.Duration
	limit    int
	keys     map[string]*rateKey
}

type rateKey struct {
	count     int
	firstTime time.Time
}

// checkRateLimit checks if the request is within rate limits.
// Returns true if allowed, false if rate limited.
func (rl *rateLimiter) checkRateLimit(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	k, ok := rl.keys[key]
	if !ok {
		k = &rateKey{firstTime: now}
		rl.keys[key] = k
	}

	// Reset counter if outside the interval
	if now.Sub(k.firstTime) > rl.interval {
		k.count = 1
		k.firstTime = now
		return true
	}

	k.count++
	if k.count > rl.limit {
		return false
	}
	return true
}

// Router holds all API routes and dependencies.
type Router struct {
	container     *service.Container
	logger        *logger.Logger
	health        *monitoring.HealthChecker
	mux           *http.ServeMux
	rateLimiter   *rateLimiter
	websocketHub  *ws.Hub
}

// NewRouter creates a new API router.
func NewRouter(container *service.Container, loggr *logger.Logger) *Router {
	r := &Router{
		container: container,
		logger:    loggr,
		health:    monitoring.NewHealthChecker(),
		mux:       http.NewServeMux(),
		rateLimiter: newRateLimiter(1 * time.Minute, 30),
		websocketHub: ws.New(container.EventBus, 30*time.Second),
	}
	r.setupHealthChecks()
	r.setupRoutes()
	return r
}

// Handler returns the HTTP handler for the API.
func (r *Router) Handler() http.Handler {
	return r.mux
}

// HealthHandler returns the health check handler.
func (r *Router) HealthHandler() http.HandlerFunc {
	return r.health.Handler()
}

// setupHealthChecks registers health checks for all dependencies.
func (r *Router) setupHealthChecks() {
	// Database health check
	r.health.AddCheck(
database, func() error {
		if r.container.DB == nil {
			return errDatabaseNil
		}
		return r.container.DB.Conn().Ping()
	})

	// MikroTik health check
	r.health.AddCheck(mikrotik, func() error {
		if r.container.Router == nil {
			return errRouterNil
		}
		health := r.container.Router.CheckHealth(nil)
		if !health.Reachable {
			return errRouterUnreachable
		}
		return nil
	})
}

func (r *Router) setupRoutes() {
	// Health endpoints (no auth)
	r.mux.HandleFunc(/health, r.health.Handler())
	r.mux.HandleFunc(/api/v1/health, r.health.Handler())

	// Public client API v1
	r.mux.HandleFunc(/api/v1/status, r.handleStatus)
	r.mux.HandleFunc(/api/v1/packages, r.handleListPackages)
	r.mux.HandleFunc(/api/v1/portal, r.handlePortalInfo)
	r.mux.HandleFunc(/api/v1/session, r.handleSessionStatus)
	r.mux.HandleFunc(/api/v1/session/connect, r.rateLimit(r.handleSessionConnect))
	r.mux.HandleFunc(/api/v1/session/disconnect, r.rateLimit(r.handleSessionDisconnect))
	r.mux.HandleFunc(/api/v1/ws, r.handleWebSocket)

	// Admin API v1 (requires authentication)
	r.mux.HandleFunc(/api/v1/admin/login, r.handleAdminLogin)
	r.mux.HandleFunc(/api/v1/admin/logout, r.requireAuth(r.handleAdminLogout))
	r.mux.HandleFunc(/api/v1/admin/check, r.requireAuth(r.handleAdminCheck))
	r.mux.HandleFunc(/api/v1/admin/devices, r.requireAuth(r.handleAdminDevices))
	r.mux.HandleFunc(/api/v1/admin/sessions, r.requireAuth(r.handleAdminSessions))
	r.mux.HandleFunc(/api/v1/admin/transactions, r.requireAuth(r.handleAdminTransactions))
	r.mux.HandleFunc(/api/v1/admin/router, r.requireAuth(r.handleAdminRouter))
}

// ServeHTTP implements http.Handler.
func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.mux.ServeHTTP(w, req)
}

// rateLimit checks rate limits for the request.
func (r *Router) rateLimit(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		key := req.RemoteAddr

		if !r.rateLimiter.checkRateLimit(key) {
			writeError(w, http.StatusTooManyRequests, rate
limit
exceeded)
			return
		}

		next(w, req)
	}
}

// handleWebSocket handles the WebSocket upgrade and connection management.
func (r *Router) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	r.websocketHub.HandleWebSocket(w, r)
}
