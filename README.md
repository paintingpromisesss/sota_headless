# Sota Headless Access-Key Server

Headless/server implementation of the Sota Connect access-key flow.

The app:

- creates and stores stable local `x-hwid` and `x-device-name` in `state/device.json`;
- calls `GET /api/v1/public/subscription/profile`;
- calls `GET /api/v1/public/connection/list`;
- selects `gate_id` by explicit value or `BST` / best server fallback;
- calls `GET /api/v1/public/connection/connect?gate_id=<id>`;
- converts the Sota sing-box snippet into a full runtime config;
- starts `sing-box run -c runtime/sing-box.runtime.json`;
- optionally exposes a local HTTP control API.

The project is implemented in Go and reads configuration from real environment
variables. In Docker Compose, `.env` is passed through `env_file`.

## Install Go Tooling

This workspace can use a local Go toolchain:

```bash
. ./scripts/go-env.sh
go version
gofmt -h
```

If Go is installed system-wide, regular `go`/`gofmt` work too.

## Quick Start

```bash
cp .env.example .env
# edit .env
docker compose up --build
```

By default `SOTA_API_ENABLED=false`, so the binary renders config, starts
`sing-box`, and keeps running. No local control API is opened.

For local smoke runs without Docker, export variables first:

```bash
set -a
. ./.env
set +a
./.tools/go/bin/go run ./cmd/sota-headless --profile
```

For a specific location:

```bash
./.tools/go/bin/go run ./cmd/sota-headless --render --gate-id 10
./.tools/go/bin/go run ./cmd/sota-headless --connect --gate-id NL
```

Direct commands:

```bash
./.tools/go/bin/go run ./cmd/sota-headless --profile
./.tools/go/bin/go run ./cmd/sota-headless --locations
./.tools/go/bin/go run ./cmd/sota-headless --render
./.tools/go/bin/go run ./cmd/sota-headless --connect
```

Running without command flags is equivalent to `--connect` unless
`SOTA_API_ENABLED=true`.

## Configuration

Required environment variable:

```env
SOTA_ACCESS_KEY=PUT_YOUR_SOTA_ACCESS_KEY_HERE
```

Common environment variables:

```env
SOTA_MODE=TUN
SOTA_API_ENABLED=false
SOTA_LISTEN=127.0.0.1:16698
SOTA_GATE_ID=
SOTA_LOG_LEVEL=info
SOTA_API_BASES=https://meowconnect.com/api/v1,https://sota.ac/api/v1

SING_BOX_BIN=
SING_BOX_DIR=./bin
SING_BOX_VERSION=v1.13.11

SOTA_PROXY_LISTEN=127.0.0.1:2080
```

`SOTA_MODE=TUN` preserves the original behavior: TUN inbound, DNS hijack,
`strict_route`, direct rule for the proxy server, and RU rule-set direct routing.
It needs administrator/root privileges or `CAP_NET_ADMIN`.

`SOTA_MODE=Proxy` creates a local sing-box `mixed` HTTP+SOCKS inbound at
`SOTA_PROXY_LISTEN` and does not manage OS routes.

`SOTA_API_ENABLED=true` starts the optional local control API instead of direct
connect mode when no command flags are provided.

## sing-box

Discovery order:

1. `SING_BOX_BIN`;
2. `sing-box` in `PATH`;
3. `SING_BOX_DIR/sing-box` or `SING_BOX_DIR/sing-box.exe`;
4. first-run download from `SagerNet/sing-box` GitHub releases using
   `SING_BOX_VERSION`.

Downloaded binaries are stored in `./bin` by default.

## HTTP API

The HTTP API is optional. Enable it with:

```env
SOTA_API_ENABLED=true
```

Default listen address: `127.0.0.1:16698`.

```text
GET  /health
GET  /status
GET  /profile
GET  /locations
GET  /device
GET  /runtime-config
GET  /runtime-config?raw=1
GET  /logs/tail
GET  /logs/tail?chars=30000
POST /render
POST /connect
POST /start
POST /disconnect
POST /stop
POST /reload
```

Examples:

```bash
curl http://127.0.0.1:16698/status
curl http://127.0.0.1:16698/locations
curl -X POST http://127.0.0.1:16698/render -H 'content-type: application/json' -d '{"gate_id":10}'
curl -X POST http://127.0.0.1:16698/connect -H 'content-type: application/json' -d '{"gate_id":10}'
curl -X POST http://127.0.0.1:16698/disconnect
```

`/runtime-config?raw=1` returns unredacted VLESS/REALITY data. Keep it local.

## Docker

```bash
cp .env.example .env
# edit .env
docker compose up --build
```

The compose file exposes:

- `16698` for the control API;
- `2080` for Proxy mode.

Ports are published on `127.0.0.1` by default, so the control API and local
proxy are not exposed to the public network. Change the host side of the port
mapping only if you intentionally want to publish them.

For `SOTA_MODE=TUN`, compose includes `NET_ADMIN` and `/dev/net/tun`. For
`SOTA_MODE=Proxy`, those privileges can be removed.

## Ubuntu Server Setup

1. Install Docker:

```bash
sudo apt-get update
sudo apt-get install -y ca-certificates curl
sudo install -m 0755 -d /etc/apt/keyrings
sudo curl -fsSL https://download.docker.com/linux/ubuntu/gpg -o /etc/apt/keyrings/docker.asc
sudo chmod a+r /etc/apt/keyrings/docker.asc
. /etc/os-release
echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.asc] https://download.docker.com/linux/ubuntu ${VERSION_CODENAME} stable" | sudo tee /etc/apt/sources.list.d/docker.list >/dev/null
sudo apt-get update
sudo apt-get install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
```

2. Prepare the app:

```bash
git clone <repo-url> sota_headless
cd sota_headless
cp .env.example .env
nano .env
```

Minimum `.env`:

```env
SOTA_ACCESS_KEY=your_real_access_key
SOTA_MODE=TUN
SOTA_API_ENABLED=false
SOTA_GATE_ID=
```

For HTTP/SOCKS proxy mode instead of TUN:

```env
SOTA_MODE=Proxy
SOTA_PROXY_LISTEN=0.0.0.0:2080
```

3. Start:

```bash
sudo docker compose up -d --build
sudo docker compose logs -f sota-headless
```

4. Check status:

```bash
sudo docker compose ps
sudo docker compose logs --tail=200 sota-headless
curl http://127.0.0.1:16698/health
```

If `SOTA_API_ENABLED=false`, the health endpoint is not started; use logs and
`docker compose ps` instead.

5. Update:

```bash
git pull
sudo docker compose up -d --build
```

For `SOTA_MODE=TUN`, the host must have `/dev/net/tun`. On most Ubuntu kernels it
already exists; otherwise run:

```bash
sudo modprobe tun
ls -l /dev/net/tun
```

## Development

```bash
. ./scripts/go-env.sh
./.tools/go/bin/gofmt -w ./cmd ./internal
./.tools/go/bin/go test ./...
./.tools/go/bin/go build -buildvcs=false ./cmd/sota-headless
```

Generated/sensitive files are ignored:

- `.env`
- `state/device.json`
- `runtime/sing-box.runtime.json`
- `runtime/sing-box.log`
- `rule_sets/*.srs`
