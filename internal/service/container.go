package service

import (
	"time"

	"github.com/unofi/unofi/internal/auth"
	"github.com/unofi/unofi/internal/bandwidth"
	"github.com/unofi/unofi/internal/captiveportal"
	"github.com/unofi/unofi/internal/database"
	"github.com/unofi/unofi/internal/device"
	"github.com/unofi/unofi/internal/events"
	"github.com/unofi/unofi/internal/logger"
	"github.com/unofi/unofi/internal/mikrotik"
	"github.com/unofi/unofi/internal/payment"
	"github.com/unofi/unofi/internal/pricing"
	"github.com/unofi/unofi/internal/session"
	"github.com/unofi/unofi/internal/voucher"
)

// Container holds all application services and dependencies.
type Container struct {
	DB                *database.DB
	Logger            *logger.Logger
	EventBus          *events.Bus
	Router            mikrotik.Router
	AuthStore         *auth.Store
	AuthRepo          auth.Repository
	DeviceRepo        device.DeviceRepository
	SessionRepo       session.SessionRepository
	TransactionRepo   payment.TransactionRepository
	VoucherRepo       voucher.Repository
	PackageRepo       pricing.Repository
	PricingEngine     *pricing.PricingEngine
	PaymentService    *payment.Service
	Portal            *captiveportal.Portal
	DiscoveryWorker   *device.DiscoveryWorker
	EnforcementWorker *session.EnforcementWorker
}

// NewContainer creates and initializes the service container.
func NewContainer(db *database.DB, loggr *logger.Logger, router mikrotik.Router, sessionTTL time.Duration) *Container {
	eventBus := events.NewBus()

	authStore := auth.NewStore(sessionTTL)
	authRepo := auth.NewSQLRepository(db)
	deviceRepo := device.NewSQLRepository(db)
	sessionRepo := session.NewSQLRepository(db)
	voucherRepo := voucher.NewSQLRepository(db)
	packageRepo := pricing.NewSQLRepository(db)
	transactionRepo := payment.NewSQLTransactionRepository(db)

	pricingEngine := pricing.NewPricingEngine(packageRepo)

	paymentService := payment.NewService(transactionRepo)

	portal := captiveportal.NewPortal(router, loggr)

	discoveryWorker := device.NewDiscoveryWorker(router, deviceRepo, eventBus, loggr, 5*time.Second)

	bandwidthMgr := bandwidth.NewManager(router)

	enforcementWorker := session.NewEnforcementWorker(
		router, sessionRepo, deviceRepo, bandwidthMgr, eventBus, loggr, 15*time.Second,
	)

	return &Container{
		DB:                db,
		Logger:            loggr,
		EventBus:          eventBus,
		Router:          router,
		AuthStore:         authStore,
		AuthRepo:          authRepo,
		DeviceRepo:        deviceRepo,
		SessionRepo:       sessionRepo,
		TransactionRepo:   transactionRepo,
		VoucherRepo:       voucherRepo,
		PackageRepo:       packageRepo,
		PricingEngine:     pricingEngine,
		PaymentService:    paymentService,
		Portal:            portal,
		DiscoveryWorker:   discoveryWorker,
		EnforcementWorker: enforcementWorker,
	}
}
