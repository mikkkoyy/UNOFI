# unofi

**Free and Open-Source WiFi Vendo Software** — rewritten in Go.

A complete rewrite of [foswvs](https://github.com/foswvs/foswvs.git) that replaces the PHP + Nginx + polling stack with a single Go binary using WebSockets for real-time updates.

## What changed from the PHP version

| Aspect | PHP original | Go rewrite |
|--------|-------------|------------|
| **Runtime** | PHP-FPM + Nginx + shell scripts | Single static binary |
| **Real-time updates** | XHR polling (1–10s intervals) | WebSocket push |
| **Database** | Raw string interpolation (SQL injection risk) | Prepared statements via `modernc.org/sqlite` |
| **Auth** | Client-side SHA256 cookie | Server-side sessions with `HttpOnly` cookies |
| **Background tasks** | `while(true)` bash loop calling PHP CLI | Goroutines with proper tickers and context cancellation |
| **Deployment** | `apt install` 6 packages + config 5 files | `scp` one binary + `systemctl enable` |
| **RAM usage** | ~80–120MB (PHP-FPM + Nginx + SQLite) | ~5–15MB |
| **Concurrency** | PHP process-per-request | Goroutines (thousands of concurrent clients) |

## Progressive Web App (PWA)

unofi is a full Progressive Web App with offline support, installable on mobile devices, and fast loading.

- **Install on home screen** — Add the app like a native app (Android/iOS)
- **Works offline** — Cached assets load instantly, even without internet
- **Service Worker** — Smart caching for speed and reliability
- **Responsive** — Perfect on any device
- **Web Push** — Future support for notifications

See [PWA.md](PWA.md) for setup, customization, and testing instructions.

### Generate PWA Icons

```bash
npm install sharp
node scripts/generate-pwa-icons.js
```

Or see [web/static/icons/README.md](web/static/icons/README.md) for manual icon generation.

## Installation

### Quick start (Raspberry Pi)

**Latest release (one-liner):**

```bash
curl -sSfL https://raw.githubusercontent.com/mikkkoyy/UNOFI/main/install.sh | bash
```

**Specific version:**

```bash
curl -sSfL https://raw.githubusercontent.com/mikkkoyy/UNOFI/main/install.sh | bash -s v1.0.0
```

Then follow [INSTALL.md](INSTALL.md) to configure the WiFi AP, DHCP server, and systemd service.

- **For full details and manual installation**, see:
  - [RELEASES.md](RELEASES.md) — available binaries and downloads
  - [INSTALL.md](INSTALL.md) — complete setup guide from scratch

## Architecture

```
┌──────────────────────────────────────────┐
│               unofi binary           │
│                                          │
│  ┌──────────┐  ┌──────────┐  ┌────────┐ │
│  │ HTTP/TLS │  │ WS Hub   │  │ SQLite │ │
│  │ Server   │◄─┤ (push)   │  │  (WAL) │ │
│  └────┬─────┘  └────┬─────┘  └───┬────┘ │
│       │              │            │      │
│  ┌────┴─────────────┴────────────┴──┐   │
│  │          Goroutines              │   │
│  │  • DHCP watcher (hook or file poll)│   │
│  │  • Usage poller (15s iptables)    │   │
│  │  • Share-tx cleaner (30s)         │   │
│  │  • Session cleanup (5m)           │   │
│  └──────────────────────────────────┘   │
│                                          │
│  ┌──────────┐  ┌──────────┐             │
│  │ iptables │  │   GPIO   │             │
│  │ (fwall)  │  │ (coins)  │             │
│  └──────────┘  └──────────┘             │
└──────────────────────────────────────────┘
```

### WebSocket message types

**Client-facing** (`/ws`):
- `data_usage` — real-time MB used/limit push (replaces 10s XHR poll)
- `network_status` — connected/disconnected state
- `topup_progress` — coin count, MB, countdown during insertion
- `topup_done` — final topup result
- `share_received` — notification when someone shares data to you
- `alert` — data exhausted, errors

**Admin-facing** (`/ws/admin`):
- `active_devices` — live list of active clients
- `earnings` — real-time earnings summary
- `system_info` — CPU temp, uptime

## Quick start (on Raspberry Pi)

Full walkthrough (flashing the OS, hostapd/DHCP setup, troubleshooting,
optional HTTPS for the QR camera scanner): **[INSTALL.md](INSTALL.md)**.

Condensed version, if you've done this before:

```bash
# On the Pi: system packages + network setup
sudo apt install -y hostapd isc-dhcp-server iptables openssl
sudo systemctl unmask hostapd
echo 'net.ipv4.ip_forward=1' | sudo tee /etc/sysctl.d/99-unofi.conf && sudo sysctl --system

# /etc/dhcpcd.conf: static IP for the AP interface
#   interface wlan0
#   static ip_address=10.0.0.1/24
#   nohook wpa_supplicant

echo 'DAEMON_CONF="/etc/hostapd/hostapd.conf"' | sudo tee -a /etc/default/isc-dhcp-server
sudo systemctl enable hostapd isc-dhcp-server

# From your own machine: cross-compile + deploy the app
#   (copies the binary, web/static, scripts/iptables-base.sh, and the
#   systemd unit, then enables + starts unofi)
make deploy PI_HOST=pi@<hostname-or-ip>
```

# Back on the Pi: bring the AP up

```bash
sudo systemctl restart dhcpcd
sudo systemctl start isc-dhcp-server hostapd
```

## First use / Admin setup

1. Connect to the `PisoWiFi` WiFi AP
2. Open any HTTP page → captive portal redirects to `10.0.0.1/`
3. Admin panel at `http://10.0.0.1/a/` — first password you enter there
   *becomes* the admin password (stored as SHA256 hash in `password.sha256`)
4. HTTPS is optional and off by default; only needed for the QR camera
   scanner in Share — see INSTALL.md if you want it

## Development

Run locally (no GPIO, no iptables — for UI development):

```powershell
go run ./cmd/main.go -addr :8080 -data-dir ./data -web-dir ./web/static
# Open http://localhost:8080
```

### Windows 404 fix

On Windows, the default data-directory path (`/home/pi/foswvs-go`) does not exist.
The server now resolves the web root as follows:

1. Use explicitly supplied `-web-dir` flag when provided.
2. Otherwise try the data-directory web path (`{dataDir}/web`).
3. If that path does not exist (e.g. Windows development), fall back to `web/static`
   next to the binary.

This ensures the existing frontend at `web/static/` is served from `http://localhost/`
during Windows development while preserving Linux/Raspberry Pi deployment behavior.

```powershell
# With explicit web-dir (recommended for Windows dev)
go run ./cmd/main.go -addr :8080 -data-dir ./data -web-dir ./web/static

# Fallback when web-dir not specified
go run ./cmd/main.go -addr :8080 -data-dir ./data
# Server falls back to web/static if ./data/web does not exist
```

## API Endpoints

### Client API

| Method | Path | Description |
|--------|------|-------------|
| `WS` | `/ws` | WebSocket for real-time client updates |
| `POST` | `/api/connect` | Open internet access |
| `GET` | `/api/data_usage` | Current MB used/limit |
| `GET` | `/api/topup` | Start coin insertion session |
| `GET` | `/api/topup_cancel` | Cancel active topup |
| `GET` | `/api/network_status` | Connection status |
| `GET` | `/api/txn` | Device transaction history |
| `GET` | `/api/rates` | Current rates |
| `POST/PUT/GET` | `/api/share` | Data sharing (create code / redeem / check) |

### Admin API (requires session cookie)

| Method | Path | Description |
|--------|------|-------------|
| `WS` | `/ws/admin` | WebSocket for admin real-time updates |
| `POST` | `/api/admin/login` | Login — first password creates admin session |
| `GET` | `/api/admin/check` | Session check — returns `status: ok` or `session expired` |
| `GET` | `/api/admin/devices?filter=` | List devices (all/active/recent/restricted) |
| `GET` | `/api/admin/device?mac=` | Device detail |
| `GET` | `/api/admin/txn` | All transactions |
| `GET` | `/api/admin/earnings` | Earnings summary |
| `GET/PUT` | `/api/admin/rates` | Get/update rates |
| `GET` | `/api/admin/system` | CPU temp, uptime |
| `GET` | `/api/admin/add_session?mac=&limit=` | Add MB to device |
| `GET` | `/api/admin/clear_mb?mac=` | Clear device data |
| `GET` | `/api/admin/block?mac=` | Block device |

## License

Same as original foswvs — open source.