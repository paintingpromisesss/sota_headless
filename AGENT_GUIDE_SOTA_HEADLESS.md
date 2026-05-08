# Agent Guide — Sota Headless Access-Key

Current version: Go rewrite.

## Purpose

This project implements a headless/server Sota Connect flow using a valid official
`SOTA_ACCESS_KEY`.

It must:

- generate and store stable device identity in `state/device.json`;
- call Sota public API endpoints:
  - `GET /api/v1/public/subscription/profile`;
  - `GET /api/v1/public/connection/list`;
  - `GET /api/v1/public/connection/connect?gate_id=<id>`;
- preserve Sota-like headers: `User-Agent`, `X-Device-Name`, `X-HWID`,
  `X-Access-Key`, `Accept-Language`, `Content-Type`;
- render full sing-box runtime config to `runtime/sing-box.runtime.json`;
- manage the sing-box process and write logs to `runtime/sing-box.log`;
- optionally expose local HTTP control endpoints.

It must not bypass authorization, derive keys, decrypt native app storage, or log
secrets.

## Layout

```text
cmd/sota-headless/      Go CLI entrypoint
internal/config/        environment loading and validation
internal/httpclient/    shared context-aware outbound HTTP client
internal/sota/          Sota API client and device identity
internal/runtime/       sing-box runtime config builder
internal/controller/    render/start/stop/reload/status orchestration
internal/server/        local HTTP API
internal/singbox/       binary discovery, download, check/run, logs
configs/                config examples
build/package/          packaging notes
scripts/                helper launch scripts
```

## Configuration

Real environment variables are authoritative. The application does not parse
`.env` itself; Docker Compose passes `.env` via `env_file`.

Required:

```env
SOTA_ACCESS_KEY=...
```

Important defaults:

```env
SOTA_MODE=TUN
SOTA_API_ENABLED=false
SOTA_LISTEN=127.0.0.1:16698
SING_BOX_VERSION=v1.13.11
SING_BOX_DIR=./bin
SOTA_PROXY_LISTEN=127.0.0.1:2080
```

`SOTA_MODE=TUN` keeps the original full-tunnel behavior. `SOTA_MODE=Proxy`
generates a local sing-box `mixed` inbound and does not manage OS routes.
`SOTA_API_ENABLED=false` means no local API is opened; default startup is direct
connect mode. `SOTA_API_ENABLED=true` starts the Fiber control API when no CLI
command flag is provided.

## Critical Runtime Rules

- All outbound HTTP requests from Go code must go through `internal/httpclient`
  and must receive a caller-provided `context.Context`.
- Never emit outbound `type: "dns"`; sing-box 1.13 removed that outbound type.
- Keep the direct route for the proxy server IP/domain to avoid routing loops.
- Keep DNS through proxy, DNS hijack, `ip_is_private -> direct`, and
  `ip.accessly.app -> proxy`.
- `/runtime-config` must redact secrets by default.
- `/runtime-config?raw=1` is local debugging only.

Sensitive values include:

```text
access_key
X-Access-Key
uuid
public_key
short_id
password
token
runtime/sing-box.runtime.json
state/device.json
```

## Development Commands

Use the workspace Go toolchain when present:

```bash
. ./scripts/go-env.sh
gofmt -w ./cmd ./internal
go test ./...
go build -buildvcs=false ./cmd/sota-headless
```

Smoke tests with environment variables exported:

```bash
set -a
. ./.env
set +a
go run ./cmd/sota-headless --profile
go run ./cmd/sota-headless --locations
go run ./cmd/sota-headless --render --gate-id BST
./bin/sing-box check -c runtime/sing-box.runtime.json
```

For TUN startup, run with administrator/root privileges or `CAP_NET_ADMIN`.
