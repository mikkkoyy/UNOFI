# Unofi Architecture

## Overview

Unofi is a WiFi hotspot vending platform designed to run on an Orange Pi (edge computer)
with a MikroTik router handling network enforcement. The system follows a clean layered
architecture that separates the application control plane from the network enforcement plane.

## Architecture Layers

### Control Plane (Orange Pi / Unofi Application)

The Unofi application is responsible for:

- **Web Portal** — Client-facing captive portal for authentication and payment
- **Admin Dashboard** — Management interface for operators
- **Authentication** — Admin session management with bcrypt password hashing
- **Device Management** — MAC/IP tracking, device identity
- **Session Management** — Data/time allocation lifecycle
- **Payment Processing** — Pluggable payment method architecture
- **Voucher System** — Voucher generation and redemption
- **Pricing Engine** — Package-based pricing with data/time grants
- **Transaction Records** — Audit trail for all payments
- **Monitoring** — Health checks and system observability
- **MikroTik Communication** — RouterOS API integration

### Network Plane (MikroTik Router)

The MikroTik router is responsible for:

- **DHCP Server** — IP address assignment
- **NAT** — Network address translation
- **Firewall** — Traffic filtering and access control
- **Hotspot** — Captive portal detection and client authorization
- **Queues** — Bandwidth enforcement per client
- **Client Authorization** — Allow/deny internet access per MAC/IP

## Package Structure

```
unofi/
├── cmd/unofi/           # Application entry point
├── internal/
│   ├── api/             # HTTP API routes, handlers, middleware
│   ├── auth/            # Authentication and session management
│   ├── bandwidth/       # Bandwidth rule management
│   ├── captiveportal/   # Captive portal operations
│   ├── config/          # Configuration and logging
│   ├── database/         # Database connection and migrations
│   ├── device/          # Device domain logic
│   ├── events/          # Event bus for decoupled communication
│   ├── mikrotik/        # MikroTik router interface and mock
│   ├── monitoring/      # Health checks
│   ├── payment/         # Payment method interface and service
│   ├── pricing/         # Pricing engine and packages
│   ├── service/         # Service container / DI
│   ├── session/         # Session lifecycle management
│   └── voucher/         # Voucher generation and redemption
├── migrations/          # Database migration files
├── web/                 # Static web assets
├── configs/             # Configuration files
├── scripts/             # Deployment scripts
├── tests/               # Test files
└── docs/                # Documentation
```

## Data Flow

### Client Connection Flow

```
1. Client connects to WiFi → MikroTik assigns DHCP lease
2. Client opens browser → MikroTik hotspot redirects to portal
3. Client views portal → Unofi serves captive portal page
4. Client makes payment → Unofi processes payment
5. Unofi creates session → Data/time allocation recorded
6. Unofi authorizes client → MikroTik grants internet access
7. Client uses data → MikroTik reports usage to Unofi
8. Session expires → Unofi deauthorizes client on MikroTik
```

### Payment Flow

```
1. Payment initiated → PaymentService.Process()
2. Payment method handler executes (Coin/Voucher/GCash/Maya)
3. Transaction recorded in database
4. Session created with data/time allocation
5. MikroTik client authorized
6. Event published to event bus
```

## MikroTik Integration

Unofi communicates with a MikroTik router via the RouterOS API for all network
enforcement. See [MIKROTIK.md](MIKROTIK.md) for detailed documentation.

Key design decisions:
- **RouterOS API Protocol** (not REST) for v6/v7 compatibility
- **HotSpot active login** for client authorization
- **Simple Queues** for bandwidth limiting
- **HotSpot active/hosts** for device discovery
- **Mock implementation** for testing without hardware

## Device Discovery

Unofi runs a background discovery worker that periodically polls MikroTik for
active HotSpot clients and synchronizes them with the Unofi device model.

### Discovery Worker

- **Location**: `internal/device/discovery.go`
- **Interval**: Configurable (default 5 seconds)
- **Data Sources**:
  - Primary: `/ip/hotspot/active/print` (authenticated clients)
  - Secondary: `/ip/hotspot/host/print` (known but unauthenticated)
- **Device Identity**: MAC address (normalized to lowercase colon-separated)
- **State Tracking**: `online`, `offline`, `last_seen`

### State Synchronization

```
MikroTik HotSpot Active
        ↓
Discovery Worker
        ↓
Unofi Device (create/update)
        ↓
Online/Offline State
        ↓
Events (device.connected, device.disconnected)
```

### Important: Discovery ≠ Authorization

Discovery only identifies devices and tracks their state. It does NOT:
- Create paid sessions
- Authorize clients for internet access
- Grant any network entitlements

Authorization happens only after payment processing through the payment flow.

## Session Enforcement

The session enforcement worker continuously evaluates active sessions,
synchronizes MikroTik usage, authorizes valid customers, and disconnects
customers whose sessions are expired, exhausted, or cancelled.

### Enforcement Worker

- **Location**: `internal/session/enforcement.go`
- **Interval**: Configurable (default 15 seconds)
- **Session Validation**: Uses `session.IsValid()` (status, expiry, exhaustion)
- **Usage Sync**: MikroTik byte counters → session MBUsed
- **Authorization**: Valid sessions → `Router.AuthorizeClient()`
- **Deauthorization**: Invalid sessions → `Router.DeauthorizeClient()`
- **Bandwidth**: Session/package bandwidth limits via `bandwidth.Manager`

### Enforcement Flow

```
             ┌──────────────────┐
             │ Payment / Voucher│
             └────────┬─────────┘
                      ↓
                ┌───────────┐
                │  Session  │
                │ Repository│
                └─────┬─────┘
                      ↓
             ┌──────────────────┐
             │ Session          │
             │ Enforcement      │
             │ Worker           │
             └───────┬──────────┘
                     ↓
              Router abstraction
                     ↓
              ┌──────────────┐
              │   MikroTik   │
              │   HotSpot    │
              └──────┬───────┘
                     ↓
                 Customer

MikroTik usage
      ↓
Discovery / Usage
      ↓
Session records
      ↓
Enforcement
```

### Session State Transitions

```
Pending
   ↓
Active
   ↓
 ┌───────────────┬──────────────┐
 ↓               ↓              ↓
Expired       Exhausted      Cancelled
```

### Router Failure Handling

If MikroTik becomes unavailable:
- Log error, skip affected operation
- Worker continues running
- Database state NOT changed to expired/exhausted
- Retry on next interval

### Discovery vs Enforcement

| Aspect | Discovery Worker | Enforcement Worker |
|--------|-----------------|-------------------|
| Purpose | Observe MikroTik clients | Enforce session state |
| Creates devices | Yes | No |
| Creates sessions | No | No |
| Authorizes clients | No | Yes (valid sessions) |
| Deauthorizes clients | No | Yes (invalid sessions) |
| Grants access | No | No (only maintains) |

## Design Principles

1. **Interface-driven design** — MikroTik, payment methods, and repositories use interfaces
   for testability and extensibility
2. **Clean separation** — Handlers → Services → Repositories → Database
3. **Dependency injection** — Service container manages all dependencies
4. **Event-driven** — Event bus for decoupled communication between components
5. **Mockable** — MikroTik and other external dependencies have mock implementations
6. **No hardcoded secrets** — Configuration from environment variables
7. **Structured logging** — Redacted sensitive data in logs

## Security

- Bcrypt password hashing (not SHA-256)
- HttpOnly, SameSite=Strict session cookies
- Constant-time password comparison
- Sensitive data redaction in logs
- No plaintext password storage

## Testing Strategy

- Mock implementations for all external dependencies
- In-memory SQLite for database tests
- Interface-based mocking for MikroTik
- No hardware required for unit tests
