#!/bin/sh
# sota-headless installer for OpenWrt
# Run on the router:
#   wget -O /tmp/install.sh https://raw.githubusercontent.com/paintingpromisesss/sota_headless/main/scripts/install.sh && sh /tmp/install.sh

set -e

REPO="paintingpromisesss/sota_headless"
GITHUB_RAW="https://raw.githubusercontent.com/${REPO}/main"
GITHUB_RELEASES="https://github.com/${REPO}/releases/latest/download"

BIN_DST="/usr/bin/sota-headless"
INIT_DST="/etc/init.d/sota-headless"
CONFIG_DIR="/etc/sota-headless"
CONFIG_FILE="${CONFIG_DIR}/sota-headless.env"
STATE_DIR="${CONFIG_DIR}/state"
DEVICE_JSON="${STATE_DIR}/device.json"

# ── Colors (yellow/grey on black) ─────────────────────────────────────────────
YEL='\033[1;33m'   # bright yellow  — headers, prompts
GRY='\033[0;37m'   # grey           — info lines
DIM='\033[2;37m'   # dim grey       — secondary text
RED='\033[0;31m'   # red            — errors
GRN='\033[0;32m'   # green          — success
NC='\033[0m'

sep()   { printf "${DIM}────────────────────────────────────────────${NC}\n"; }
hdr()   { printf "\n${YEL}  %s${NC}\n" "$1"; sep; }
info()  { printf "${GRY}  • %s${NC}\n" "$1"; }
ok()    { printf "${GRN}  ✓ %s${NC}\n" "$1"; }
warn()  { printf "${YEL}  ! %s${NC}\n" "$1"; }
error() { printf "${RED}  ✗ %s${NC}\n" "$1"; exit 1; }
ask()   { printf "${YEL}  → %s${NC} " "$1"; }

# ── Detect arch ───────────────────────────────────────────────────────────────
detect_arch() {
    case "$(uname -m)" in
        aarch64)        echo "linux-arm64"  ;;
        armv7l|armv6l)  echo "linux-arm"    ;;
        x86_64)         echo "linux-amd64"  ;;
        mips)           echo "linux-mips"   ;;
        mipsel|mipsle)  echo "linux-mipsle" ;;
        *)              error "Unsupported arch: $(uname -m). Build manually." ;;
    esac
}

# ── Check deps ────────────────────────────────────────────────────────────────
check_deps() {
    for cmd in wget chmod; do
        command -v "$cmd" >/dev/null 2>&1 || error "Missing: $cmd"
    done
}

# ── Download ──────────────────────────────────────────────────────────────────
download() {
    URL="$1"; DST="$2"
    info "Fetching $(basename "$DST") ..."
    wget -q -O "$DST" "$URL" || error "Download failed: $URL"
}

# ── Check if service already running ─────────────────────────────────────────
check_existing() {
    UPGRADING=0
    if [ -x "$INIT_DST" ]; then
        if "$INIT_DST" status >/dev/null 2>&1; then
            warn "sota-headless is already running — this is an upgrade"
            UPGRADING=1
        else
            warn "sota-headless init script exists but service is stopped"
            UPGRADING=1
        fi
    fi

    if [ -f "$DEVICE_JSON" ]; then
        info "Found existing device.json — will preserve device identity"
        PRESERVE_DEVICE=1
    else
        PRESERVE_DEVICE=0
    fi
}

# ── Access key dialog ─────────────────────────────────────────────────────────
ask_access_key() {
    # Read current key from config if exists
    CURRENT_KEY=""
    if [ -f "$CONFIG_FILE" ]; then
        CURRENT_KEY=$(grep '^SOTA_ACCESS_KEY=' "$CONFIG_FILE" 2>/dev/null | cut -d= -f2-)
        # Ignore placeholder
        [ "$CURRENT_KEY" = "PUT_YOUR_SOTA_ACCESS_KEY_HERE" ] && CURRENT_KEY=""
    fi

    hdr "Sota access key"

    if [ -n "$CURRENT_KEY" ]; then
        info "Current key: ${GRY}${CURRENT_KEY}${NC}"
        printf "${DIM}  Leave blank to keep it, or enter a new key to replace.${NC}\n"
        ask "New access key (Enter to keep):"
        read INPUT_KEY
        if [ -z "$INPUT_KEY" ]; then
            ACCESS_KEY="$CURRENT_KEY"
            info "Keeping existing access key"
        else
            ACCESS_KEY="$INPUT_KEY"
            ok "Access key updated"
        fi
    else
        printf "${DIM}  UUID format: xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx${NC}\n"
        ask "Access key:"
        read INPUT_KEY
        [ -z "$INPUT_KEY" ] && error "Access key cannot be empty"
        ACCESS_KEY="$INPUT_KEY"
        ok "Access key set"
    fi

    # Soft format check
    case "$ACCESS_KEY" in
        ????????-????-????-????-????????????) ;;
        *) warn "Key format looks unusual — continuing anyway" ;;
    esac
}

# ── Write config ──────────────────────────────────────────────────────────────
write_config() {
    mkdir -p "$CONFIG_DIR" "$STATE_DIR"

    # Preserve any custom overrides from existing config (HWID, DEVICE_NAME, etc.)
    OLD_HWID=""
    OLD_DEVICE_NAME=""
    OLD_CACHE_TTL="30m"
    OLD_LISTEN="0.0.0.0:16698"
    if [ -f "$CONFIG_FILE" ]; then
        OLD_HWID=$(grep '^SOTA_HWID=' "$CONFIG_FILE" 2>/dev/null | cut -d= -f2-)
        OLD_DEVICE_NAME=$(grep '^SOTA_DEVICE_NAME=' "$CONFIG_FILE" 2>/dev/null | cut -d= -f2-)
        OLD_CACHE_TTL=$(grep '^SOTA_CACHE_TTL=' "$CONFIG_FILE" 2>/dev/null | cut -d= -f2-)
        OLD_LISTEN=$(grep '^SOTA_LISTEN=' "$CONFIG_FILE" 2>/dev/null | cut -d= -f2-)
        [ -z "$OLD_CACHE_TTL" ] && OLD_CACHE_TTL="30m"
        [ -z "$OLD_LISTEN" ]    && OLD_LISTEN="0.0.0.0:16698"
    fi

    cat > "$CONFIG_FILE" << EOF
# sota-headless configuration
# Updated: $(date -Iseconds 2>/dev/null || date)

SOTA_ACCESS_KEY=${ACCESS_KEY}
SOTA_BASE_DIR=${CONFIG_DIR}
SOTA_LISTEN=${OLD_LISTEN}
SOTA_CACHE_TTL=${OLD_CACHE_TTL}
EOF

    # Re-append optional overrides only if they were set before
    if [ -n "$OLD_HWID" ]; then
        echo "SOTA_HWID=${OLD_HWID}" >> "$CONFIG_FILE"
    fi
    if [ -n "$OLD_DEVICE_NAME" ]; then
        echo "SOTA_DEVICE_NAME=${OLD_DEVICE_NAME}" >> "$CONFIG_FILE"
    fi

    chmod 600 "$CONFIG_FILE"
    ok "Config saved to $CONFIG_FILE"
}

# ── Main ──────────────────────────────────────────────────────────────────────
main() {
    printf "\n${YEL}════════════════════════════════════════════${NC}\n"
    printf "${YEL}    sota-headless  ·  OpenWrt installer${NC}\n"
    printf "${YEL}════════════════════════════════════════════${NC}\n"

    check_deps
    ARCH=$(detect_arch)
    info "Architecture: $ARCH"

    check_existing

    ask_access_key

    hdr "Downloading"

    # Stop service before upgrade
    if [ "$UPGRADING" = "1" ] && [ -x "$INIT_DST" ]; then
        info "Stopping existing service..."
        "$INIT_DST" stop 2>/dev/null || true
        sleep 1
    fi

    # Binary
    TMP_BIN="/tmp/sota-headless"
    download "${GITHUB_RELEASES}/sota-headless-${ARCH}" "$TMP_BIN"
    chmod +x "$TMP_BIN"
    mv "$TMP_BIN" "$BIN_DST"
    ok "Binary → $BIN_DST"

    # Init script
    download "${GITHUB_RAW}/scripts/sota-headless.init" "$INIT_DST"
    chmod +x "$INIT_DST"
    ok "Init script → $INIT_DST"

    # Config
    write_config

    # device.json: already preserved if it existed (we never touched it)
    if [ "$PRESERVE_DEVICE" = "1" ]; then
        ok "Device identity preserved ($DEVICE_JSON)"
    else
        info "Device identity will be generated on first start"
    fi

    hdr "Starting service"
    "$INIT_DST" enable
    "$INIT_DST" start
    sleep 2

    if wget -q -O /dev/null "http://127.0.0.1:16698/health" 2>/dev/null; then
        ok "Service is up and responding!"
    else
        warn "Service started but /health not responding yet — may still be initializing"
        info "Check: logread | grep sota"
    fi

    # Detect LAN IP on OpenWrt (br-lan or uci network.lan.ipaddr)
    ROUTER_IP=$(uci get network.lan.ipaddr 2>/dev/null || ip -4 addr show br-lan 2>/dev/null | awk '/inet /{print $2}' | cut -d/ -f1 | head -n1)
    [ -z "$ROUTER_IP" ] && ROUTER_IP=$(ip -4 addr show lan 2>/dev/null | awk '/inet /{print $2}' | cut -d/ -f1 | head -n1)
    [ -z "$ROUTER_IP" ] && ROUTER_IP=$(ip route get 1 2>/dev/null | awk '{print $7; exit}')
    [ -z "$ROUTER_IP" ] && ROUTER_IP="192.168.0.1"

    printf "\n${YEL}════════════════════════════════════════════${NC}\n"
    printf "${YEL}    Done!${NC}\n"
    printf "${YEL}════════════════════════════════════════════${NC}\n\n"
    printf "${GRY}  Subscription URLs:${NC}\n\n"
    printf "  ${YEL}Mihomo / Zashboard${NC}\n"
    printf "  ${DIM}http://${ROUTER_IP}:16698/sub/mihomo${NC}\n\n"
    printf "  ${YEL}Base64 (universal)${NC}\n"
    printf "  ${DIM}http://${ROUTER_IP}:16698/sub/base64${NC}\n\n"
    printf "  ${YEL}Plain vless:// links${NC}\n"
    printf "  ${DIM}http://${ROUTER_IP}:16698/sub/vless${NC}\n\n"
    printf "${GRY}  Health:  ${DIM}curl http://127.0.0.1:16698/health${NC}\n"
    printf "${GRY}  Logs:    ${DIM}logread | grep sota${NC}\n\n"
}

main "$@"
