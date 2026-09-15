package bandwidth

import (
	"context"
	"fmt"

	"github.com/unofi/unofi/internal/mikrotik"
)

// Manager handles bandwidth rule management through the MikroTik router.
type Manager struct {
	router mikrotik.Bandwidth
}

// NewManager creates a new bandwidth manager.
func NewManager(router mikrotik.Bandwidth) *Manager {
	return &Manager{router: router}
}

// CreateLimit creates a bandwidth limit rule for an IP address.
func (m *Manager) CreateLimit(ctx context.Context, ip, maxLimit string) error {
	rule := &mikrotik.BandwidthRule{
		Name:     fmt.Sprintf("limit-%s", ip),
		TargetIP: ip,
		MaxLimit: maxLimit,
		Enabled:  true,
	}
	return m.router.CreateRule(ctx, rule)
}

// UpdateLimit modifies an existing bandwidth limit rule.
func (m *Manager) UpdateLimit(ctx context.Context, ruleID, maxLimit string) error {
	rule := &mikrotik.BandwidthRule{
		ID:       ruleID,
		MaxLimit: maxLimit,
		Enabled:  true,
	}
	return m.router.UpdateRule(ctx, rule)
}

// RemoveLimit deletes a bandwidth limit rule.
func (m *Manager) RemoveLimit(ctx context.Context, ruleID string) error {
	return m.router.RemoveRule(ctx, ruleID)
}

// GetLimits returns all bandwidth limit rules.
func (m *Manager) GetLimits(ctx context.Context) ([]*mikrotik.BandwidthRule, error) {
	return m.router.GetRules(ctx)
}
