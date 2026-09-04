<div align="center">

# sota-headless

**Headless Sota Connect subscription provider for OpenWrt routers**

[![Go](https://img.shields.io/badge/Go-1.26-00ADD8?style=flat&logo=go&logoColor=white)](https://go.dev)
[![Release](https://img.shields.io/github/v/release/paintingpromisesss/sota_headless?style=flat&color=yellow)](https://github.com/paintingpromisesss/sota_headless/releases/latest)
[![License](https://img.shields.io/badge/license-MIT-grey?style=flat)](LICENSE)
[![OpenWrt](https://img.shields.io/badge/OpenWrt-25.x-00B5E2?style=flat&logo=openwrt&logoColor=white)](https://openwrt.org)

**English** • [Русский](README.md)

Sota Connect lock their subscriptions inside proprietary applications.  
This tool authenticates with their API and serves the nodes as a standard subscription — directly from your router.

</div>

---

## How it works

```
Your device (Happ / Mihomo / v2rayNG / Zashboard / ...)
        │
        │  GET http://router:16698/sub/mihomo
        ▼
  sota-headless  (background daemon on your router)
        │
        │  X-Access-Key + X-HWID → Sota API
        ▼
   meowconnect.com  →  VLESS + Reality nodes
```

Nodes are cached for 30 minutes. Zero external dependencies, no built-in sing-box, ~8 MB binary.

---

## Install on OpenWrt

Connect to your router via SSH and run:

```sh
wget -O /tmp/install.sh https://raw.githubusercontent.com/paintingpromisesss/sota_headless/main/scripts/install.sh && sh /tmp/install.sh
```

The installer will:
- detect your router's CPU architecture automatically
- prompt for your Sota Access Key
- download the binary and register the procd service
- start the service and print your subscription URLs

### Supported routers

| Chip / Platform | Architecture | Example devices |
|-----------------|--------------|-----------------|
| MediaTek Filogic (MT7981/MT7986) | `arm64` | Netis NX31, GL.iNet MT3000 |
| Rockchip RK3568 | `arm64` | NanoPi R5S, R5C |
| MediaTek MT7621 | `mipsle` | Xiaomi R3G, Keenetic Giga |
| Atheros AR9xxx | `mips` | TP-Link WR1043 |
| x86/x64 | `amd64` | PC Engines APU, Mini-PCs |

---

## Subscription URLs

After installation, use the corresponding URL in your proxy client:

| Format | URL | Client / Target |
|--------|-----|-----------------|
| **Mihomo YAML** | `http://router-ip:16698/sub/mihomo` | Mihomo, Zashboard, Clash.Meta |
| **Base64** | `http://router-ip:16698/sub/base64` | Happ, v2rayNG, Shadowrocket |
| **Plain vless://** | `http://router-ip:16698/sub/vless` | Manual import |
| **sing-box JSON** | `http://router-ip:16698/sub/singbox` | sing-box outbounds array |

> Replace `router-ip` with your router's local IP address (e.g., `192.168.0.1` or `192.168.1.1`).

---

## Configuration

The configuration file is stored at `/etc/sota-headless/sota-headless.env`:

```env
SOTA_ACCESS_KEY=your-access-key-here
SOTA_LISTEN=0.0.0.0:16698
SOTA_CACHE_TTL=30m
# Log level: debug, info (default), warn, error
SOTA_LOG_LEVEL=info
```

Restart the service after making changes:

```sh
service sota-headless restart
```

To force-refresh the node cache without restarting:

```sh
curl -X POST http://127.0.0.1:16698/sub/refresh
```

---

## Update

Simply re-run the installer — it will detect the existing setup, preserve your access key and device identity, and update the binary:

```sh
wget -O /tmp/install.sh https://raw.githubusercontent.com/paintingpromisesss/sota_headless/main/scripts/install.sh && sh /tmp/install.sh
```

---

## Useful commands

```sh
# Service status
service sota-headless status

# Check logs
logread | grep sota

# Health check
curl http://127.0.0.1:16698/health

# Subscription profile details (expiry, quota)
curl http://127.0.0.1:16698/profile

# List available locations
curl http://127.0.0.1:16698/locations
```

---

<div align="center">
<sub>Not affiliated with Cat Connect Oy / Sota Connect. Use at your own discretion.</sub>
</div>
