package monitoring

import (
	"encoding/json"
	"net/http"
	"time"
)

// HealthChecker performs health checks on system dependencies.
type HealthChecker struct {
	checks map[string]HealthCheck
}

// HealthCheck is a function that checks the health of a dependency.
type HealthCheck func() error

// HealthResponse represents the overall health status.
type HealthResponse struct {
	Status    string                 `json:"status"`
	Timestamp time.Time              `json:"timestamp"`
	Checks    map[string]CheckResult `json:"checks"`
}

// CheckResult represents the result of an individual health check.
type CheckResult struct {
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

// NewHealthChecker creates a new health checker.
func NewHealthChecker() *HealthChecker {
	return &HealthChecker{
		checks: make(map[string]HealthCheck),
	}
}

// AddCheck registers a health check for a component.
func (h *HealthChecker) AddCheck(name string, check HealthCheck) {
	h.checks[name] = check
}

// Check runs all health checks and returns the overall status.
func (h *HealthChecker) Check() HealthResponse {
	response := HealthResponse{
		Status:    "healthy",
		Timestamp: time.Now(),
		Checks:    make(map[string]CheckResult),
	}

	for name, check := range h.checks {
		result := CheckResult{Status: "healthy"}
		if err := check(); err != nil {
			result.Status = "unhealthy"
			result.Error = err.Error()
			response.Status = "unhealthy"
		}
		response.Checks[name] = result
	}

	return response
}

// Handler returns an HTTP handler for the health endpoint.
func (h *HealthChecker) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		response := h.Check()
		
		w.Header().Set("Content-Type", "application/json")
		if response.Status != "healthy" {
			w.WriteHeader(http.StatusServiceUnavailable)
		} else {
			w.WriteHeader(http.StatusOK)
		}
		
		json.NewEncoder(w).Encode(response)
	}
}
