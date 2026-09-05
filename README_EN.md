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

Nodes are cached for 30 minutes. Zero external dependencies, no built-in sing-box, ~7 MB binary.

---

## Installation

### Quick install
 
**For Windows (PowerShell as Administrator):**
```powershell
irm https://raw.githubusercontent.com/paintingpromisesss/sota_headless/main/scripts/install.ps1 | iex
```

**For Linux (Ubuntu / Debian / CentOS / Arch / VPS / servers):**
```sh
curl -fsSL https://raw.githubusercontent.com/paintingpromisesss/sota_headless/main/scripts/install.sh | sudo sh
```

**For OpenWrt (routers):**
```sh
wget -O /tmp/install.sh https://raw.githubusercontent.com/paintingpromisesss/sota_headless/main/scripts/install.sh && sh /tmp/install.sh
```

The installer will:
- detect your platform (Windows, Linux with `systemd`, or OpenWrt with `procd`) and CPU architecture
- prompt for your Sota Access Key
- download the binary and set up the system service (on Windows: native Windows service and firewall rule)
- start the service and print your ready-to-use subscription URLs

### Lightweight UPX version for Flash-constrained routers (16–32 MB)

If your router has limited flash memory, a UPX-compressed variant is available:

**For OpenWrt (routers):**
```sh
wget -O /tmp/install.sh https://raw.githubusercontent.com/paintingpromisesss/sota_headless/main/scripts/install.sh && SOTA_UPX=1 sh /tmp/install.sh
```

**For Linux:**
```sh
curl -fsSL https://raw.githubusercontent.com/paintingpromisesss/sota_headless/main/scripts/install.sh | sudo SOTA_UPX=1 sh
```

**For Windows (PowerShell):**
```powershell
$env:SOTA_UPX=1; irm https://raw.githubusercontent.com/paintingpromisesss/sota_headless/main/scripts/install.ps1 | iex
```

> #### ⚖️ Version comparison:
>
> | Metric | Standard version | UPX-compressed version | Difference |
> |---|---|---|---|
> | **Size on disk / Flash** | ~6.0 – 6.7 MB | **~1.7 – 2.0 MB** | **~70–75% Flash savings** |
> | **RAM usage (RSS)** | **~10 – 14 MB** | ~16 – 22 MB | **Overhead: +5–7 MB RAM** |
> | **Cold start time** | ~5 ms | ~30–60 ms | +30–50 ms decompressor time |

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
 
The configuration file is stored at:
- **Windows**: `C:\Program Files\sota-headless\sota-headless.env`
- **Linux / OpenWrt**: `/etc/sota-headless/sota-headless.env`

```env
SOTA_ACCESS_KEY=your-access-key-here
SOTA_LISTEN=0.0.0.0:16698
SOTA_CACHE_TTL=30m
# Log level: debug, info (default), warn, error
SOTA_LOG_LEVEL=info
```

Restart the service after making changes:

- **Windows (PowerShell)**: `Restart-Service sota-headless`
- **Linux**: `systemctl restart sota-headless`
- **OpenWrt**: `service sota-headless restart`

To force-refresh the node cache without restarting:

```sh
curl -X POST http://127.0.0.1:16698/sub/refresh
```

---

## Update

Simply re-run the installer — it will detect the existing setup, preserve your access key and device identity, and update the binary.

---

## Useful commands

```sh
# Service management:
# Windows (PowerShell as Administrator):
Get-Service sota-headless
Restart-Service sota-headless
Stop-Service sota-headless

# Linux (systemd):
systemctl status sota-headless
systemctl restart sota-headless
journalctl -u sota-headless -f

# OpenWrt (procd):
service sota-headless status
service sota-headless restart
logread -f -e sota

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
