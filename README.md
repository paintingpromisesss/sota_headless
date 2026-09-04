<div align="center">

# sota-headless

**Headless Sota Connect subscription provider for OpenWrt routers**

[![Go](https://img.shields.io/badge/Go-1.26-00ADD8?style=flat&logo=go&logoColor=white)](https://go.dev)
[![Release](https://img.shields.io/github/v/release/paintingpromisesss/sota_headless?style=flat&color=yellow)](https://github.com/paintingpromisesss/sota_headless/releases/latest)
[![License](https://img.shields.io/badge/license-MIT-grey?style=flat)](LICENSE)
[![OpenWrt](https://img.shields.io/badge/OpenWrt-25.x-00B5E2?style=flat&logo=openwrt&logoColor=white)](https://openwrt.org)

Sota Connect lock their subscriptions behind proprietary apps.  
This tool authenticates with their API and serves the nodes as a standard subscription — directly from your router.

</div>

---

## How it works

```
Your device (Happ / Mihomo / v2rayNG / ...)
        │
        │  GET http://router:16698/sub/mihomo
        ▼
  sota-headless  (running on your router)
        │
        │  X-Access-Key + X-HWID → Sota API
        ▼
   meowconnect.com  →  VLESS + Reality nodes
```

Nodes are cached for 30 minutes. No external dependencies, no sing-box, ~8 MB binary.

---

## Install on OpenWrt

SSH into your router and run:

```sh
wget -O /tmp/install.sh https://raw.githubusercontent.com/paintingpromisesss/sota_headless/main/scripts/install.sh && sh /tmp/install.sh
```

The installer will:
- detect your router's CPU architecture automatically
- ask for your Sota access key
- download the binary and set up the service
- start everything and print your subscription URLs

### Supported routers

| Chip | Architecture | Example devices |
|------|-------------|-----------------|
| MediaTek Filogic (MT7981/MT7986) | `arm64` | Netis NX31, GL.iNet MT3000 |
| Rockchip RK3568 | `arm64` | NanoPi R5S, R5C |
| MediaTek MT7621 | `mipsle` | Xiaomi R3G, Keenetic Giga |
| Atheros AR9xxx | `mips` | TP-Link WR1043 |
| x86/x64 | `amd64` | PC Engines APU |

---

## Subscription URLs

After install, use these in your proxy client:

| Format | URL | Use with |
|--------|-----|----------|
| **Mihomo YAML** | `http://router-ip:16698/sub/mihomo` | Mihomo, Zashboard, Clash.Meta |
| **Base64** | `http://router-ip:16698/sub/base64` | Happ, v2rayNG, Shadowrocket |
| **Plain vless://** | `http://router-ip:16698/sub/vless` | Manual import |
| **sing-box JSON** | `http://router-ip:16698/sub/singbox` | sing-box outbounds |

> Replace `router-ip` with your router's LAN address (e.g. `192.168.0.1`).

---

## Configuration

Config is stored at `/etc/sota-headless/sota-headless.env`:

```env
SOTA_ACCESS_KEY=your-key-here
SOTA_LISTEN=0.0.0.0:16698
SOTA_CACHE_TTL=30m
```

After editing, restart the service:

```sh
service sota-headless restart
```

To force-refresh the node cache without restarting:

```sh
curl -X POST http://127.0.0.1:16698/sub/refresh
```

---

## Update

Re-run the same installer — it will detect the existing installation, preserve your access key and device identity, and update the binary:

```sh
wget -O /tmp/install.sh https://raw.githubusercontent.com/paintingpromisesss/sota_headless/main/scripts/install.sh && sh /tmp/install.sh
```

---

## Useful commands

```sh
# Check service status
service sota-headless status

# View logs
logread | grep sota

# Check health
curl http://127.0.0.1:16698/health

# View current subscription profile
curl http://127.0.0.1:16698/profile

# List all available locations
curl http://127.0.0.1:16698/locations
```

---

<div align="center">
<sub>Not affiliated with Sota Connect. Use at your own discretion.</sub>
</div>
