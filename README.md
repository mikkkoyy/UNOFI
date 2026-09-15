# Unofi

**Free and Open-Source WiFi Vendo Platform**

Unofi is the application/control plane for WiFi hotspot vending. It runs on an
Orange Pi (edge computer) and communicates with a MikroTik router (network
enforcement plane) to deliver a complete pay-as-you-go internet solution.

## Architecture

- **Orange Pi** — web portal, admin dashboard, authentication, payments, vouchers, pricing, monitoring
- **MikroTik** — DHCP, NAT, firewall, hotspot, client authorization, bandwidth enforcement, queues

## Quick Start

```bash
go build -o unofi ./cmd/unofi
./unofi -config configs/unofi.yaml
```

## License

Open Source
