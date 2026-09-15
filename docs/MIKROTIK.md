# MikroTik RouterOS Integration

## Overview

Unofi communicates with a MikroTik router via the RouterOS API to manage network
enforcement. The Orange Pi runs Unofi as the control plane, while the MikroTik
handles all network-level operations (DHCP, NAT, firewall, hotspot, queues).

## Architecture

Customer Internet traffic does NOT pass through the Orange Pi.

```
INTERNET
   │
   ▼
MIKROTIK
   │
   ▼
AP / SWITCH
   │
   ▼
CUSTOMER

        ┌──────────────┐
        │  ORANGE PI   │
        │    UNOFI     │
        └──────┬───────┘
               │
          RouterOS API
               │
               ▼
           MIKROTIK
```

The Orange Pi is the **control plane** (management, payments, sessions).
The MikroTik is the **data plane** (traffic, DHCP, NAT, firewall).
┌──────────────────────────────────────────────────────────────────┐
│                        Orange Pi (Unofi)                          │
│                                                                   │
│  ┌──────────┐   ┌──────────┐   ┌──────────┐   ┌──────────┐      │
│  │ Web      │   │ Admin    │   │ Payment  │   │ Session  │      │
│  │ Portal   │   │ API      │   │ Engine   │   │ Manager  │      │
│  └────┬─────┘   └────┬─────┘   └────┬─────┘   └────┬─────┘      │
│       │              │              │              │              │
│  ┌────┴──────────────┴──────────────┴──────────────┴─────┐       │
│  │              Service Container                         │       │
│  └──────────────────────┬────────────────────────────────┘       │
│                         │                                        │
│  ┌──────────────────────┴────────────────────────────────┐       │
│  │          MikroTik Client (RouterOS API)                │       │
│  │                                                       │       │
│  │  ┌──────────┐  ┌──────────┐  ┌──────────┐            │       │
│  │  │ Client   │  │ Bandwidth│  │ System   │            │       │
│  │  │ Ops      │  │ Ops      │  │ Ops      │            │       │
│  │  └──────────┘  └──────────┘  └──────────┘            │       │
│  └──────────────────────┬────────────────────────────────┘       │
└─────────────────────────┼────────────────────────────────────────┘
                          │
                    RouterOS API (TCP 8728/8729)
                          │
┌─────────────────────────┼────────────────────────────────────────┐
│                   MikroTik Router                                │
│                         │                                        │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐        │
│  │ HotSpot  │  │ Queue    │  │ DHCP     │  │ Firewall │        │
│  │ Active   │  │ Simple   │  │ Server   │  │ NAT      │        │
│  └──────────┘  └──────────┘  └──────────┘  └──────────┘        │
└──────────────────────────────────────────────────────────────────┘
```

## Connection Method

Unofi uses the **RouterOS API Protocol** (not REST) for maximum compatibility
with both RouterOS v6 and v7.

- **Port 8728**: Standard API (plain TCP)
- **Port 8729**: API-SSL (TLS encrypted)

The connection is established using the `go-routeros/routeros/v3` library,
which implements the RouterOS binary API protocol.

### Connection Lifecycle

1. **Connect**: Dial TCP + login with username/password
2. **Operations**: Send commands via `RunContext()`
3. **Health Check**: Periodic `/system/resource/print` to verify connectivity
4. **Reconnect**: Automatic reconnect on connection loss
5. **Close**: Graceful shutdown with `Close()`

## Authentication

RouterOS API uses username/password authentication. Unofi sends credentials
during the initial connection handshake.

### Required Permissions

The Unofi API user needs the following permissions:

| Permission | Purpose |
|------------|---------|
| `read` | Query system info, client lists, queues |
| `write` | Add/remove queue rules |
| `api` | Access the API interface |
| `hotspot` | Manage HotSpot active users |

**Recommended**: Create a dedicated user group on MikroTik with only the
required permissions. Do not use the `full` admin account for Unofi.

### Creating the API User

```routeros
/user group add name=unofi policy=read,write,api,hotspot
/user add name=unofi password=<strong_password> group=unofi
```

## Client Authorization Mechanism

Unofi uses **HotSpot active login** to authorize clients for internet access.

### Flow

1. Client connects to WiFi → gets IP via DHCP
2. Client opens browser → HotSpot redirects to captive portal
3. Client makes payment → Unofi processes payment
4. Unofi calls `/ip/hotspot/active/login` with client MAC and IP
5. MikroTik authorizes the client → internet access granted

### RouterOS Commands Used

| Operation | RouterOS Command |
|-----------|-----------------|
| Authorize | `/ip/hotspot/active/login` |
| Deauthorize | `/ip/hotspot/active/remove` |
| Disconnect | `/ip/hotspot/active/remove` + `/ip/hotspot/host/remove` |
| List Active | `/ip/hotspot/active/print` |
| List Hosts | `/ip/hotspot/host/print` |

## Bandwidth Mechanism

Unofi uses **Simple Queues** for bandwidth limiting per client.

### Why Simple Queues?

- Simple to manage (add/remove per client)
- Per-IP targeting
- Dynamic max-limit adjustment
- Automatic cleanup when client disconnects

### Queue Naming Convention

Queues are named: `unofi-<MAC>` (e.g., `unofi-aa:bb:cc:dd:ee:ff`)

### RouterOS Commands Used

| Operation | RouterOS Command |
|-----------|-----------------|
| Create | `/queue/simple/add` |
| Update | `/queue/simple/set` |
| Remove | `/queue/simple/remove` |
| List | `/queue/simple/print` |

## Usage Tracking

Client data usage is retrieved from the HotSpot active user table.

### Data Available

- `bytes-in`: Download bytes (32-bit, may overflow)
- `bytes-in64`: Download bytes (64-bit, preferred)
- `bytes-out`: Upload bytes (32-bit, may overflow)
- `bytes-out64`: Upload bytes (64-bit, preferred)
- `uptime`: Connection uptime
- `address`: Client IP
- `mac-address`: Client MAC

**Note**: Unofi prefers 64-bit counters (`bytes-in64`/`bytes-out64`) and falls
back to 32-bit if 64-bit values are not available.

## Device Discovery

Unofi discovers clients through two MikroTik data sources:

### 1. HotSpot Active (`/ip/hotspot/active/print`)

Authenticated clients with internet access. This is the primary source for
active session tracking.

### 2. HotSpot Hosts (`/ip/hotspot/host/print`)

Known clients (DHCP leases that have been seen by HotSpot) but not yet
authenticated. Used to show pending/unauthenticated devices.

### Why Not ARP or DHCP Leases?

- **ARP**: Too low-level, doesn't correlate with HotSpot authentication state
- **DHCP Leases**: Includes all DHCP clients, not just WiFi/HotSpot users
- **HotSpot Active/Hosts**: Directly maps to the Unofi authentication model

### Discovery Worker

The discovery worker (`internal/device/discovery.go`) runs as a background
goroutine that periodically polls MikroTik and synchronizes client state.

**Key behaviors:**
- Creates new Unofi devices for newly discovered MACs
- Updates IP/hostname for existing devices
- Tracks `online`/`offline` state based on presence in active list
- Updates `last_seen` timestamp on each discovery
- Publishes `device.connected` and `device.disconnected` events
- Handles RouterOS failures gracefully (logs, retries next interval)

**Configuration:**
- `UNOFI_DISCOVERY_ENABLED` - Enable/disable discovery (default: true)
- `UNOFI_DISCOVERY_INTERVAL` - Polling interval (default: 5s)

### MAC Address Normalization

All MAC addresses are normalized to lowercase colon-separated format:
- `AA:BB:CC:DD:EE:FF` → `aa:bb:cc:dd:ee:ff`
- `AA-BB-CC-DD-EE-FF` → `aa:bb:cc:dd:ee:ff`
- `aabb.ccdd.eeff` → `aa:bb:cc:dd:ee:ff`
- `aabbccddeeff` → `aa:bb:cc:dd:ee:ff`

This prevents duplicate devices from formatting differences.

### Discovery ≠ Authorization

**Critical**: Discovery only identifies devices and tracks state. It does NOT:
- Create paid sessions
- Authorize clients for internet access
- Grant any network entitlements

The correct flow is:
```
MikroTik client discovered
        ↓
Unofi Device created/updated
        ↓
Check existing Session
        ↓
If valid entitlement exists → authorization/enforcement
```

Authorization happens only after payment processing through the payment flow.

## System Information

Retrieved via `/system/resource/print` and `/system/identity/print`:

| Field | Source | Description |
|-------|--------|-------------|
| Identity | `/system/identity` | Router name |
| Version | `/system/resource` | RouterOS version |
| Model | `/system/resource` | Board name |
| Uptime | `/system/resource` | System uptime |
| CPU Usage | `/system/resource` | CPU load % |
| Memory | `/system/resource` | Total/free memory |

## Health Check

The health check performs a lightweight `/system/resource/print` command and
measures response latency.

- **Reachable**: Command succeeds
- **Unreachable**: Command fails or times out
- **Latency**: Round-trip time for the health check command

On failure, the client marks itself as disconnected and will attempt to
reconnect on the next operation.

## Failure and Reconnect Behavior

1. **Connection Lost**: Operations fail with "not connected" error
2. **Auto-Reconnect**: Next operation triggers `ensureConnected()` → `Connect()`
3. **Health Check**: Periodic health checks detect connection state
4. **Graceful Degradation**: Unofi continues running even if MikroTik is unreachable

## Security Considerations

### What Unofi Does NOT Modify

- WAN configuration
- DNS settings (global)
- Firewall filter rules (default)
- NAT rules (global)
- DHCP server configuration
- Wireless configuration
- RouterOS system settings
- Other users/groups

### What Unofi DOES Modify

- HotSpot active user list (login/remove)
- Simple queue rules (add/remove/set)

### Best Practices

1. **Use API-SSL** (port 8729) in production to encrypt credentials
2. **Create a dedicated user** with minimal permissions
3. **Use strong passwords** for the API user
4. **Restrict API access** by IP if possible (Firewall filter on MikroTik)
5. **Monitor API access** via MikroTik logs

## Configuration

```yaml
mikrotik:
  address: "192.168.1.1"    # Router IP
  port: 8728                 # API port (8728 plain, 8729 SSL)
  username: "unofi"          # Dedicated API user
  password: "strong_password" # API user password
  use_tls: false             # true for API-SSL
  timeout: "10s"             # Connection/operation timeout
```

### Environment Variables

```bash
export UNOFI_MIKROTIK_ADDRESS=192.168.1.1
export UNOFI_MIKROTIK_PORT=8728
export UNOFI_MIKROTIK_USERNAME=unofi
export UNOFI_MIKROTIK_PASSWORD=strong_password
export UNOFI_MIKROTIK_TLS=false
```

## Required RouterOS Configuration

For Unofi to work, the MikroTik router must have:

1. **API Service Enabled**:
   ```routeros
   /ip service set api port=8728
   /ip service set api-ssl port=8729 disabled=no
   ```

2. **HotSpot Configured** on the WiFi interface with:
   - IP HotSpot with login-by=http-chap or similar
   - User profile with appropriate settings

3. **API User Created** with hotspot and policy permissions

4. **Firewall Rules** allowing API access from Orange Pi IP

## Troubleshooting

| Issue | Cause | Solution |
|-------|-------|----------|
| Connection refused | API service disabled | Enable `/ip service set api disabled=no` |
| Login failed | Wrong credentials | Check username/password |
| Timeout | Network issue | Verify IP/port, check firewall |
| No clients shown | HotSpot not configured | Check HotSpot setup on interface |
| Queue not working | Queue disabled | Check `/queue/simple` settings |
