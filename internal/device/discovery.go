package device

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/unofi/unofi/internal/events"
	"github.com/unofi/unofi/internal/logger"
	"github.com/unofi/unofi/internal/mikrotik"
)

// DiscoveryWorker periodically polls MikroTik for active clients and
// synchronizes them with the Unofi device model.
type DiscoveryWorker struct {
	mu         sync.Mutex
	router     mikrotik.Router
	repo       DeviceRepository
	eventBus   *events.Bus
	logger     *logger.Logger
	interval   time.Duration
	running    bool
	cancelFunc context.CancelFunc
}

// NewDiscoveryWorker creates a new discovery worker.
func NewDiscoveryWorker(
	router mikrotik.Router,
	repo DeviceRepository,
	eventBus *events.Bus,
	loggr *logger.Logger,
	interval time.Duration,
) *DiscoveryWorker {
	return &DiscoveryWorker{
		router:   router,
		repo:     repo,
		eventBus: eventBus,
		logger:   loggr,
		interval: interval,
	}
}

// Start begins the discovery worker loop.
func (w *DiscoveryWorker) Start(ctx context.Context) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.running {
		return fmt.Errorf("discovery worker already running")
	}

	ctx, cancel := context.WithCancel(ctx)
	w.cancelFunc = cancel
	w.running = true

	w.logger.Info("device discovery worker started (interval=%s)", w.interval)

	go w.run(ctx)

	return nil
}

// Stop gracefully stops the discovery worker.
func (w *DiscoveryWorker) Stop() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if !w.running {
		return
	}

	if w.cancelFunc != nil {
		w.cancelFunc()
	}
	w.running = false
	w.logger.Info("device discovery worker stopped")
}

// IsRunning returns true if the worker is currently running.
func (w *DiscoveryWorker) IsRunning() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.running
}

// run is the main worker loop.
func (w *DiscoveryWorker) run(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	// Run immediately on start
	w.discover(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.discover(ctx)
		}
	}
}

// discover performs a single discovery cycle.
func (w *DiscoveryWorker) discover(ctx context.Context) {
	w.logger.Debug("device discovery cycle starting")

	// Get active clients from MikroTik
	activeClients, err := w.router.GetClients(ctx)
	if err != nil {
		w.logger.Warn("device discovery failed: %v", err)
		return
	}

	// Build a set of currently active MACs
	activeMACs := make(map[string]bool)

	for _, client := range activeClients {
		mac := NormalizeMAC(client.MAC)
		if mac == "" {
			continue
		}

		activeMACs[mac] = true

		// Create or update the device
		device, err := w.repo.GetByMAC(mac)
		if err != nil {
			w.logger.Error("get device by mac %s: %v", mac, err)
			continue
		}

		if device == nil {
			// New device discovered
			device, err = w.repo.Create(mac, client.IP, client.Hostname)
			if err != nil {
				w.logger.Error("create device mac=%s: %v", mac, err)
				continue
			}
			w.logger.Info("new device discovered: mac=%s ip=%s", mac, client.IP)

			// Publish event
			if w.eventBus != nil {
				w.eventBus.Publish(events.Event{
					Type: events.EventDeviceConnected,
					Data: device,
				})
			}
		} else {
			// Update existing device
			// Update IP if changed
			if client.IP != "" && client.IP != device.IP {
				if err := w.repo.Update(device.ID, client.IP, device.Hostname); err != nil {
					w.logger.Error("update device ip: %v", err)
				}
			}

			// Update hostname if changed
			if client.Hostname != "" && client.Hostname != device.Hostname && client.Hostname != "-NA-" {
				if err := w.repo.Update(device.ID, device.IP, client.Hostname); err != nil {
					w.logger.Error("update device hostname: %v", err)
				}
			}

			// Update last seen
			if err := w.repo.UpdateLastSeen(device.ID); err != nil {
				w.logger.Error("update device last_seen: %v", err)
			}

			// Mark online if was offline
			if !device.Online {
				if err := w.repo.SetOnline(device.ID, true); err != nil {
					w.logger.Error("set device online: %v", err)
				}

				w.logger.Info("device online: mac=%s ip=%s", mac, client.IP)

				// Publish event
				if w.eventBus != nil {
					w.eventBus.Publish(events.Event{
						Type: events.EventDeviceConnected,
						Data: device,
					})
				}
			}
		}
	}

	// Mark devices not in active list as offline
	w.markOfflineDevices(ctx, activeMACs)

	w.logger.Debug("device discovery cycle complete (active=%d)", len(activeMACs))
}

// markOfflineDevices marks devices not in the active set as offline.
func (w *DiscoveryWorker) markOfflineDevices(ctx context.Context, activeMACs map[string]bool) {
	onlineDevices, err := w.repo.ListOnline()
	if err != nil {
		w.logger.Error("list online devices: %v", err)
		return
	}

	for _, device := range onlineDevices {
		mac := NormalizeMAC(device.MAC)
		if !activeMACs[mac] {
			// Device no longer active
			if err := w.repo.SetOnline(device.ID, false); err != nil {
				w.logger.Error("set device offline mac=%s: %v", device.MAC, err)
				continue
			}

			w.logger.Info("device offline: mac=%s", mac)

			// Publish event
			if w.eventBus != nil {
				w.eventBus.Publish(events.Event{
					Type: events.EventDeviceDisconnected,
					Data: device,
				})
			}
		}
	}
}

// NormalizeMAC normalizes a MAC address to lowercase colon-separated format.
// Accepts:
//
//	AA:BB:CC:DD:EE:FF
//	aa:bb:cc:dd:ee:ff
//	AA-BB-CC-DD-EE-FF
//	aabb.ccdd.eeff
//	aabbccddeeff
//
// Returns empty string if the input is not a valid MAC.
func NormalizeMAC(mac string) string {
	if mac == "" {
		return ""
	}

	// Remove separators and convert to lowercase
	cleaned := strings.Map(func(r rune) rune {
		if r == ':' || r == '-' || r == '.' {
			return -1
		}
		return r
	}, strings.ToLower(mac))

	// Must be exactly 12 hex characters
	if len(cleaned) != 12 {
		return ""
	}

	// Validate hex
	for _, c := range cleaned {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return ""
		}
	}

	// Insert colons
	return cleaned[0:2] + ":" + cleaned[2:4] + ":" + cleaned[4:6] + ":" +
		cleaned[6:8] + ":" + cleaned[8:10] + ":" + cleaned[10:12]
}
