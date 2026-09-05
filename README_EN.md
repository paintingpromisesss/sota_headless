<div align="center">

# sota-headless

**Sota Connect subscription generation service**

[![Go](https://img.shields.io/badge/Go-1.26-00ADD8?style=flat&logo=go&logoColor=white)](https://go.dev)
[![Release](https://img.shields.io/github/v/release/paintingpromisesss/sota_headless?style=flat&color=yellow)](https://github.com/paintingpromisesss/sota_headless/releases/latest)
[![License](https://img.shields.io/badge/license-MIT-grey?style=flat)](LICENSE)
[![OpenWrt](https://img.shields.io/badge/OpenWrt-25.x-00B5E2?style=flat&logo=openwrt&logoColor=white)](https://openwrt.org)

**English** • [Русский](README.md)

Local service for fetching Sota Connect node configurations via API and serving them as standard subscriptions for third-party clients.

</div>

---

## Architecture Diagram

```
Client (Mihomo, v2rayNG, Happ, sing-box, etc.)
        │
        │  GET http://<host>:16698/sub/<format>
        ▼
  sota-headless (local service)
        │
        │  API requests (X-Access-Key, X-HWID)
        ▼
  Sota Connect API  →  VLESS + Reality node parameters
```

The service does not route user traffic. It queries node parameters via API, formats subscriptions as requested, and caches the result in RAM (30 minutes by default).

---

## Installation

### Standard installation

**Windows (PowerShell as Administrator):**
```powershell
irm https://raw.githubusercontent.com/paintingpromisesss/sota_headless/main/scripts/install.ps1 | iex
```

**Linux (Ubuntu, Debian, CentOS, Arch, servers):**
```sh
curl -fsSL https://raw.githubusercontent.com/paintingpromisesss/sota_headless/main/scripts/install.sh | sudo sh
```

**OpenWrt (routers):**
```sh
wget -O /tmp/install.sh https://raw.githubusercontent.com/paintingpromisesss/sota_headless/main/scripts/install.sh && sh /tmp/install.sh
```

### UPX build (for memory-constrained devices)

For routers with limited storage space, a UPX-compressed build is available:

**OpenWrt:**
```sh
wget -O /tmp/install.sh https://raw.githubusercontent.com/paintingpromisesss/sota_headless/main/scripts/install.sh && SOTA_UPX=1 sh /tmp/install.sh
```

**Linux:**
```sh
curl -fsSL https://raw.githubusercontent.com/paintingpromisesss/sota_headless/main/scripts/install.sh | sudo SOTA_UPX=1 sh
```

**Windows (PowerShell):**
```powershell
$env:SOTA_UPX=1; irm https://raw.githubusercontent.com/paintingpromisesss/sota_headless/main/scripts/install.ps1 | iex
```

> #### Build variant comparison
>
> | Metric | Standard build | UPX build | Notes |
> |---|---|---|---|
> | Binary size on disk | ~6.0–6.7 MB | ~1.7–2.0 MB | ~70% storage space savings |
> | RAM usage (RSS) | ~10–14 MB | ~16–22 MB | Additional ~5–7 MB RAM overhead |
> | Startup time | ~5 ms | ~30–60 ms | Decompression overhead during process start |

### Supported router platforms

| Platform / SoC | Architecture | Example devices |
|----------------|--------------|-----------------|
| MediaTek Filogic (MT7981/MT7986) | `arm64` | Netis NX31, GL.iNet MT3000 |
| Rockchip RK3568 | `arm64` | NanoPi R5S, R5C |
| MediaTek MT7621 | `mipsle` | Xiaomi R3G, Keenetic Giga |
| Atheros AR9xxx | `mips` | TP-Link WR1043 |
| x86-64 | `amd64` | PC Engines APU, x86 routers, mini PCs |

---

## Subscription Formats

Once the service is running, enter the appropriate URL into your client configuration:

| Format | URL | Supported clients |
|--------|-----|-------------------|
| **Mihomo YAML** | `http://<host>:16698/sub/mihomo` | Mihomo, Clash.Meta, Zashboard |
| **Base64** | `http://<host>:16698/sub/base64` | Happ, v2rayNG, Shadowrocket, Streisand |
| **Line-delimited VLESS** | `http://<host>:16698/sub/vless` | Direct import of `vless://` links |
| **sing-box JSON** | `http://<host>:16698/sub/singbox` | `outbounds` array for sing-box configuration |

> `<host>` is the local network IP of the machine or `127.0.0.1` if the client runs on the same system.

---

## Configuration

Settings are defined in the environment configuration file:
- **Windows**: `C:\Program Files\sota-headless\sota-headless.env`
- **Linux / OpenWrt**: `/etc/sota-headless/sota-headless.env`

```env
SOTA_ACCESS_KEY=your-access-key-here
SOTA_LISTEN=0.0.0.0:16698
SOTA_CACHE_TTL=30m
# Log level: debug, info (default), warn, error
SOTA_LOG_LEVEL=info
```

Restart the service to apply changes:

- **Windows (PowerShell)**: `Restart-Service sota-headless`
- **Linux**: `systemctl restart sota-headless`
- **OpenWrt**: `service sota-headless restart`

To force a cache refresh without restarting:

```sh
curl -X POST http://127.0.0.1:16698/sub/refresh
```

---

## Updating

To update, rerun the installation command. The script will replace the executable with the latest version while preserving existing settings and the device identity file (`device.json`).

---

## Management and Diagnostics

```sh
# Service management:
# Windows (PowerShell):
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

# Health check:
curl http://127.0.0.1:16698/health

# Service status and cache information:
curl http://127.0.0.1:16698/status

# Subscription details (expiry, quota):
curl http://127.0.0.1:16698/profile

# Available locations:
curl http://127.0.0.1:16698/locations
```

---

## Technical Details

The service runs as a local HTTP server that translates Sota Connect API responses into standard subscription formats.

### Local API Endpoints

#### Subscriptions

| Method and path | Response format | Purpose |
|---|---|---|
| `GET /sub`<br>`GET /sub/mihomo` | YAML (`text/yaml`) | `proxy-provider` format subscription for Mihomo and Clash.Meta |
| `GET /sub/vless` | Plain text (`text/plain`) | Line-delimited list of `vless://` links |
| `GET /sub/base64` | Base64 (`text/plain`) | Base64-encoded `vless://` links list |
| `GET /sub/singbox` | JSON (`application/json`) | JSON with `outbounds` array for sing-box |

#### Information and Status

| Method and path | Response format | Purpose |
|---|---|---|
| `GET /`<br>`GET /health` | JSON (`application/json`) | Health check, version, and basic summary |
| `GET /status` | JSON (`application/json`) | Service state: cached nodes count, cache age, TTL, active API base, errors |
| `GET /profile` | JSON (`application/json`) | Subscription info from Sota API (status, expiry, limits) |
| `GET /locations` | JSON (`application/json`) | List of available locations and servers from Sota API |
| `GET /device` | JSON (`application/json`) | Registered identity (`x_device_name` and masked `x_hwid`) |

#### Management

| Method and path | Response format | Purpose |
|---|---|---|
| `POST /sub/refresh` | JSON (`application/json`) | Invalidate local cache and immediately fetch fresh nodes from Sota API |

---

### 1. Device Initialization and Identification

On first run, the service creates a local device profile:
- A configuration file is saved at `state/device.json` with `x_hwid` and `x_device_name`.
- `x_hwid` is generated as a SHA-256 hash of the hostname, operating system, platform architecture, and 32 random bytes.
- `x_device_name` defaults to `<hostname>_<os>_<arch>`.
- Both parameters can be overridden using `SOTA_HWID` and `SOTA_DEVICE_NAME` environment variables.

### 2. Sota Connect API Communication

Requests to the API include the following headers:
- `X-Access-Key` — user subscription key;
- `X-HWID` — device identifier;
- `X-Device-Name` — registered device name;
- `User-Agent` — client identifier (emulates the official application by default).

### 3. Node Collection and Parsing

Nodes are fetched in two steps:
1. **Locations list**: `GET /public/connection/list` returns available gateways with their `gate_id`, names, and country codes.
2. **Connection parameters**: `GET /public/connection/connect?gate_id=<id>` is executed for each `gate_id`.

From each gateway response, VLESS + Reality connection parameters are extracted:
- Server address (`server`) and port (`server_port`);
- User ID (`uuid`);
- Reality parameters: public key (`public_key`), short ID (`short_id`), SNI (`server_name`), TLS fingerprint (`fingerprint`), and flow mode (`flow`).

Parsed nodes are normalized into an internal `Node` structure and sorted by `gate_id`.

### 4. In-Memory Caching

- Parsed nodes are kept in RAM.
- Caching avoids re-querying nodes on every client request and reduces API load.
- Cache expiration is controlled by `SOTA_CACHE_TTL` (default is 30 minutes). Once expired, the next client request triggers a refresh from the API.
- The cache can be invalidated manually via `POST /sub/refresh`.

### 5. System Resources

- The service only formats subscriptions and does not route network traffic.
- Runtime memory usage is capped at 48 MB (`debug.SetMemoryLimit(48 MiB)`) to prevent OOM termination on resource-constrained routers.

---

<div align="center">
<sub>Not affiliated with Cat Connect Oy / Sota Connect. Provided "as is".</sub>
</div>
