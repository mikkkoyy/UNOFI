package captiveportal

import (
	"github.com/unofi/unofi/internal/logger"
	"github.com/unofi/unofi/internal/mikrotik"
)

// Portal manages captive portal operations.
type Portal struct {
	router  mikrotik.Router
	logger  *logger.Logger
}

// NewPortal creates a new captive portal manager.
func NewPortal(router mikrotik.Router, loggr *logger.Logger) *Portal {
	return &Portal{
		router: router,
		logger: loggr,
	}
}

// AuthorizeDevice authorizes a device for internet access through the captive portal.
func (p *Portal) AuthorizeDevice(mac, ip string) error {
	p.logger.Info("authorizing device: mac=%s ip=%s", mac, ip)
	return p.router.AuthorizeClient(nil, mac, ip)
}

// DeauthorizeDevice removes a device's authorization.
func (p *Portal) DeauthorizeDevice(mac string) error {
	p.logger.Info("deauthorizing device: mac=%s", mac)
	return p.router.DeauthorizeClient(nil, mac)
}

// DisconnectDevice forcibly disconnects a device.
func (p *Portal) DisconnectDevice(mac string) error {
	p.logger.Info("disconnecting device: mac=%s", mac)
	return p.router.DisconnectClient(nil, mac)
}

// GetClients returns all captive portal clients.
func (p *Portal) GetClients() ([]*mikrotik.Client, error) {
	return p.router.GetClients(nil)
}

// GetClientByMAC returns a specific client by MAC.
func (p *Portal) GetClientByMAC(mac string) (*mikrotik.Client, error) {
	return p.router.GetClientByMAC(nil, mac)
}
