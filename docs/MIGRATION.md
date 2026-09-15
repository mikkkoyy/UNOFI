# Migration Guide: foswvs-go to Unofi

## Overview

Unofi is a ground-up architectural redesign of the foswvs-go WiFi vending system. While the
core business domain remains the same (WiFi hotspot vending with coin/voucher payment), the
architecture has been completely restructured for modularity, testability, and extensibility.

## What Changed

### Architecture

| Aspect | foswvs-go (old) | Unofi (new) |
|--------|-----------------|-------------|
| Structure | Monolithic handlers | Modular packages with clean boundaries |
| Networking | Linux iptables directly | MikroTik router via interface abstraction |
| Database | Raw SQL in handlers | Repository pattern with interfaces |
| Auth | SHA-256 password hashing | Bcrypt with secure session management |
| Payments | Hardcoded coin acceptor | Pluggable payment method interface |
| Testing | No tests | Full test foundation with mocks |
| API | Unversioned `/api/` | Versioned `/api/v1/` |
| Configuration | CLI flags + env vars | Centralized config with env override |
| Logging | Standard log package | Structured logging with redaction |
| Frontend | Inline JS in HTML | Separated (future: build system) |

### What Was Reused

The following concepts and patterns from foswvs-go were preserved:

1. **Device/Session/Transaction model** — Core domain entities with similar fields
2. **MAC-based device identity** — Devices tracked by MAC address with IP tracking
3. **Session lifecycle** — Data/time allocation with expiration
4. **Captive portal concept** — Redirect unauthenticated users to payment portal
5. **DHCP lease monitoring** — Tracking client connections via DHCP events
6. **Piso-based pricing** — Currency-to-data conversion via configurable rates
7. **WebSocket real-time updates** — Push notifications to connected clients
8. **Maintenance modes** — Lockdown and free-data modes
9. **Data sharing** — Client-to-client data transfer via codes
10. **PWA approach** — Installable web app with service worker

### What Was Replaced

1. **iptables with MikroTik** — Network enforcement moved to the router
2. **Coin acceptor GPIO** — Abstracted behind PaymentMethod interface
3. **SHA-256 passwords** — Replaced with bcrypt
4. **Single handler file** — Split into domain-specific packages
5. **Global state** — Moved to dependency injection container
6. **In-memory sessions only** — Now persisted to database
7. **No tests** — Full test foundation with mocks

### What Is Deferred

The following foswvs-go features are NOT yet implemented in Unofi:

1. **WebSocket server** — API structure exists, WS handlers to be added
2. **Client captive portal page** — Static file serving exists, UI to be built
3. **Admin dashboard UI** — Admin API exists, frontend to be built
4. **Coin acceptor implementation** — PaymentMethod interface exists, coin handler to be built
5. **Real MikroTik client** — Interface and mock exist, RouterOS client to be built
6. **Bandwidth shaping via tc** — Replaced with MikroTik queue management
7. **Data sharing between clients** — Share code logic to be ported
8. **Maintenance mode UI/API** — Foundation exists, full feature to be built
9. **Hotspot integration** — MikroTik hotspot to be implemented
10. **Email/SMS notifications** — Notification service structure to be added

## Database Migration

The Unofi database schema is a superset of the foswvs-go schema:

- `devices` — Same core fields, added `last_seen_at`
- `sessions` — Added `voucher_id`, `payment_id`, `time_limit_seconds`, `time_used_seconds`, `expires_at`
- `transactions` — New table replacing the implicit session-as-transaction model
- `vouchers` — New table for voucher management
- `packages` — New table for configurable pricing packages
- `admin_users` — New table with bcrypt password hashes
- `admin_sessions` — New table for persisted admin sessions

## API Migration

| foswvs-go | Unofi |
|-----------|-------|
| `/api/connect` | `/api/v1/client/connect` (future) |
| `/api/topup` | `/api/v1/client/topup` (future) |
| `/api/admin/login` | `/api/v1/admin/login` |
| `/api/admin/devices` | `/api/v1/admin/devices` |
| `/ws` | `/ws` (future) |
| N/A | `/health` |
| N/A | `/api/v1/health` |

## Configuration Migration

| foswvs-go | Unofi |
|-----------|-------|
| `-addr` | `server.host` + `server.port` |
| `-data-dir` | `database.path` |
| `-iface` | Removed (MikroTik handles networking) |
| `-dspeed`/`-uspeed` | Removed (MikroTik handles shaping) |
| `FOSWVS_DEV` | `UNOFI_LOG_LEVEL=debug` |
| N/A | `UNOFI_MIKROTIK_ADDRESS` |
| N/A | `UNOFI_AUTH_SECRET` |

## Deployment Migration

| Aspect | foswvs-go | Unofi |
|--------|-----------|-------|
| Hardware | Raspberry Pi | Orange Pi PC |
| Network | Linux iptables/nftables | MikroTik router |
| Install | `scp binary + systemctl` | Same pattern (future) |
| Database | SQLite | SQLite (same) |
| Real-time | WebSocket | WebSocket (future) |

## Business Logic Reference

The following business semantics were preserved from foswvs-go:

### Session Model

| Concept | foswvs-go | Unofi |
|---------|-----------|-------|
| Session creation | `AddSession(deviceID, amount, mbLimit)` | `sessionRepo.Create()` |
| Usage tracking | `UpdateMBUsed(deviceID, mb)` | `sessionRepo.UpdateUsage()` |
| Data exhaustion | `MBLimit <= MBUsed` | `session.IsExhausted()` |
| Auto-reconnect | DHCP watcher checks remaining data | Enforcement worker authorizes valid sessions |
| Session extension | New session adds MB to device | New session creates separate record |

### Usage Polling

| Aspect | foswvs-go | Unofi |
|--------|-----------|-------|
| Mechanism | `iptables -nvxL FORWARD` byte counters | `Router.GetClientUsage()` via HotSpot API |
| Interval | 15 seconds | 15 seconds (configurable) |
| Counter type | iptables bytes | HotSpot bytes-in64/bytes-out64 |
| Disconnection | `iptables -D FORWARD` | `Router.DeauthorizeClient()` |

### Session Enforcement

| Aspect | foswvs-go | Unofi |
|--------|-----------|-------|
| Authorization | `iptables -A FORWARD -s IP -j ACCEPT` | `Router.AuthorizeClient()` |
| Deauthorization | `iptables -D FORWARD` | `Router.DeauthorizeClient()` |
| Exhaustion check | `MBLimit <= MBUsed` | `session.IsExhausted()` |
| Time-based expiry | Not implemented | `session.IsExpired()` via `ExpiresAt` |
| Background loop | `UsagePoller` goroutine | `EnforcementWorker` goroutine |
