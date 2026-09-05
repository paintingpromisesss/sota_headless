#!/bin/sh
# sota-headless installer for Linux / OpenWrt
# Run on your server or router:
#   curl -fsSL https://raw.githubusercontent.com/paintingpromisesss/sota_headless/main/scripts/install.sh | sudo sh
# or (OpenWrt):
#   wget -O /tmp/install.sh https://raw.githubusercontent.com/paintingpromisesss/sota_headless/main/scripts/install.sh && sh /tmp/install.sh

set -e

REPO="paintingpromisesss/sota_headless"
GITHUB_RAW="https://raw.githubusercontent.com/${REPO}/main"

# Default to latest stable release, or use explicit SOTA_VERSION if provided (e.g. SOTA_VERSION=v1.1.0)
if [ -n "$SOTA_VERSION" ] && [ "$SOTA_VERSION" != "latest" ]; then
    case "$SOTA_VERSION" in
        v*) ;;
        *)  SOTA_VERSION="v${SOTA_VERSION}" ;;
    esac
    GITHUB_RELEASES="https://github.com/${REPO}/releases/download/${SOTA_VERSION}"
    INSTALL_VERSION="$SOTA_VERSION"
else
    GITHUB_RELEASES="https://github.com/${REPO}/releases/latest/download"
    INSTALL_VERSION="latest"
fi

BIN_DST="/usr/bin/sota-headless"
INIT_DST="/etc/init.d/sota-headless"
SYSTEMD_SERVICE="/etc/systemd/system/sota-headless.service"
CONFIG_DIR="/etc/sota-headless"
CONFIG_FILE="${CONFIG_DIR}/sota-headless.env"
STATE_DIR="${CONFIG_DIR}/state"
DEVICE_JSON="${STATE_DIR}/device.json"

# ── Colors (yellow/grey on black) ─────────────────────────────────────────────
ESC=$(printf '\033')
YEL="${ESC}[1;33m"   # bright yellow  — headers, prompts
GRY="${ESC}[0;37m"   # grey           — info lines
DIM="${ESC}[2;37m"   # dim grey       — secondary text
RED="${ESC}[0;31m"   # red            — errors
GRN="${ESC}[0;32m"   # green          — success
NC="${ESC}[0m"

sep()   { printf "%b────────────────────────────────────────────%b\n" "$DIM" "$NC"; }
hdr()   { printf "\n%b  %s%b\n" "$YEL" "$1" "$NC"; sep; }
info()  { printf "%b  • %s%b\n" "$GRY" "$1" "$NC"; }
ok()    { printf "%b  ✓ %s%b\n" "$GRN" "$1" "$NC"; }
warn()  { printf "%b  ! %s%b\n" "$YEL" "$1" "$NC"; }
error() { printf "%b  ✗ %s%b\n" "$RED" "$1" "$NC"; exit 1; }
ask()   { printf "%b  → %s%b " "$YEL" "$1" "$NC"; }

# Read input safely even when script is piped via curl/wget | sh
read_input() {
    if [ -t 0 ]; then
        read -r "$1"
    elif [ -e /dev/tty ]; then
        read -r "$1" </dev/tty
    else
        read -r "$1"
    fi
}

# ── Check root ────────────────────────────────────────────────────────────────
check_root() {
    if [ "$(id -u 2>/dev/null || true)" != "0" ]; then
        error "This script must be run as root. Please run with sudo or as root."
    fi
}

# ── Detect init system ────────────────────────────────────────────────────────
detect_init() {
    if [ -f /etc/openwrt_release ] || [ -f /etc/init.d/rc.common ]; then
        INIT_SYSTEM="procd"
    elif command -v systemctl >/dev/null 2>&1 && [ -d /run/systemd/system ]; then
        INIT_SYSTEM="systemd"
    else
        INIT_SYSTEM="unknown"
    fi
}

# ── Language selection ────────────────────────────────────────────────────────
select_language() {
    printf "\n%b════════════════════════════════════════════%b\n" "$YEL" "$NC"
    printf "%b    sota-headless  ·  Linux / OpenWrt installer%b\n" "$YEL" "$NC"
    printf "%b════════════════════════════════════════════%b\n\n" "$YEL" "$NC"
    printf "%b  [1] English (default)\n  [2] Русский%b\n\n" "$GRY" "$NC"
    ask "Select language / Выберите язык [1/2]:"
    read_input LANG_CHOICE

    case "$LANG_CHOICE" in
        2|[rR][uU])
            LANG_UI="ru"
            ;;
        *)
            LANG_UI="en"
            ;;
    esac
}

# ── Detect arch ───────────────────────────────────────────────────────────────
detect_arch() {
    case "$(uname -m)" in
        aarch64|arm64)  echo "linux-arm64"  ;;
        armv7l|armv6l)  echo "linux-arm"    ;;
        x86_64|amd64)   echo "linux-amd64"  ;;
        mips)           echo "linux-mips"   ;;
        mipsel|mipsle)  echo "linux-mipsle" ;;
        *)
            if [ "$LANG_UI" = "ru" ]; then
                error "Неподдерживаемая архитектура: $(uname -m). Скомпилируйте вручную."
            else
                error "Unsupported arch: $(uname -m). Build manually."
            fi
            ;;
    esac
}

# ── Check deps ────────────────────────────────────────────────────────────────
check_deps() {
    if ! command -v wget >/dev/null 2>&1 && ! command -v curl >/dev/null 2>&1; then
        if [ "$LANG_UI" = "ru" ]; then
            error "Отсутствует wget или curl. Пожалуйста, установите один из них."
        else
            error "Neither wget nor curl is installed. Please install one of them."
        fi
    fi
    for cmd in chmod mkdir; do
        if ! command -v "$cmd" >/dev/null 2>&1; then
            if [ "$LANG_UI" = "ru" ]; then
                error "Отсутствует необходимая утилита: $cmd"
            else
                error "Missing required utility: $cmd"
            fi
        fi
    done
}

# ── Download ──────────────────────────────────────────────────────────────────
download() {
    URL="$1"; DST="$2"
    if [ "$LANG_UI" = "ru" ]; then
        info "Загрузка $(basename "$DST") ..."
    else
        info "Fetching $(basename "$DST") ..."
    fi
    if command -v wget >/dev/null 2>&1; then
        wget -q -O "$DST" "$URL" || {
            if [ "$LANG_UI" = "ru" ]; then
                error "Ошибка загрузки: $URL"
            else
                error "Download failed: $URL"
            fi
        }
    elif command -v curl >/dev/null 2>&1; then
        curl -fsSL -o "$DST" "$URL" || {
            if [ "$LANG_UI" = "ru" ]; then
                error "Ошибка загрузки: $URL"
            else
                error "Download failed: $URL"
            fi
        }
    fi
}

# ── Check if service already running ─────────────────────────────────────────
check_existing() {
    UPGRADING=0
    if [ "$INIT_SYSTEM" = "systemd" ]; then
        if [ -f "$SYSTEMD_SERVICE" ]; then
            if systemctl is-active --quiet sota-headless 2>/dev/null; then
                if [ "$LANG_UI" = "ru" ]; then
                    warn "Служба sota-headless (systemd) запущена — выполняется обновление"
                else
                    warn "sota-headless (systemd) service is running — this is an upgrade"
                fi
                UPGRADING=1
            else
                if [ "$LANG_UI" = "ru" ]; then
                    warn "Служба sota-headless (systemd) существует, но остановлена"
                else
                    warn "sota-headless (systemd) service exists but is stopped"
                fi
                UPGRADING=1
            fi
        fi
    elif [ "$INIT_SYSTEM" = "procd" ]; then
        if [ -x "$INIT_DST" ]; then
            if "$INIT_DST" status >/dev/null 2>&1; then
                if [ "$LANG_UI" = "ru" ]; then
                    warn "Служба sota-headless (procd) уже запущена — выполняется обновление"
                else
                    warn "sota-headless (procd) is already running — this is an upgrade"
                fi
                UPGRADING=1
            else
                if [ "$LANG_UI" = "ru" ]; then
                    warn "Скрипт службы существует, но сервис остановлен"
                else
                    warn "sota-headless init script exists but service is stopped"
                fi
                UPGRADING=1
            fi
        fi
    fi

    if [ -f "$DEVICE_JSON" ]; then
        if [ "$LANG_UI" = "ru" ]; then
            info "Найден существующий device.json — идентификатор устройства сохранён"
        else
            info "Found existing device.json — will preserve device identity"
        fi
        PRESERVE_DEVICE=1
    else
        PRESERVE_DEVICE=0
    fi
}

# ── Access key dialog ─────────────────────────────────────────────────────────
ask_access_key() {
    CURRENT_KEY=""
    if [ -f "$CONFIG_FILE" ]; then
        CURRENT_KEY=$(grep '^SOTA_ACCESS_KEY=' "$CONFIG_FILE" 2>/dev/null | cut -d= -f2-)
        [ "$CURRENT_KEY" = "PUT_YOUR_SOTA_ACCESS_KEY_HERE" ] && CURRENT_KEY=""
    fi

    if [ "$LANG_UI" = "ru" ]; then
        hdr "Ключ доступа Sota"
    else
        hdr "Sota access key"
    fi

    if [ -n "$CURRENT_KEY" ]; then
        if [ "$LANG_UI" = "ru" ]; then
            info "Текущий ключ: $CURRENT_KEY"
            printf "%b  Оставьте пустым, чтобы сохранить текущий ключ, или введите новый.%b\n" "$DIM" "$NC"
            ask "Новый ключ (Enter чтобы оставить):"
        else
            info "Current key: $CURRENT_KEY"
            printf "%b  Leave blank to keep it, or enter a new key to replace.%b\n" "$DIM" "$NC"
            ask "New access key (Enter to keep):"
        fi
        read_input INPUT_KEY
        if [ -z "$INPUT_KEY" ]; then
            ACCESS_KEY="$CURRENT_KEY"
            if [ "$LANG_UI" = "ru" ]; then
                info "Сохранён текущий ключ доступа"
            else
                info "Keeping existing access key"
            fi
        else
            ACCESS_KEY="$INPUT_KEY"
            if [ "$LANG_UI" = "ru" ]; then
                ok "Ключ доступа обновлён"
            else
                ok "Access key updated"
            fi
        fi
    else
        if [ "$LANG_UI" = "ru" ]; then
            printf "${DIM}  Формат UUID: xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx${NC}\n"
            ask "Ключ доступа:"
        else
            printf "${DIM}  UUID format: xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx${NC}\n"
            ask "Access key:"
        fi
        read_input INPUT_KEY
        if [ -z "$INPUT_KEY" ]; then
            if [ "$LANG_UI" = "ru" ]; then
                error "Ключ доступа не может быть пустым"
            else
                error "Access key cannot be empty"
            fi
        fi
        ACCESS_KEY="$INPUT_KEY"
        if [ "$LANG_UI" = "ru" ]; then
            ok "Ключ доступа установлен"
        else
            ok "Access key set"
        fi
    fi

    case "$ACCESS_KEY" in
        ????????-????-????-????-????????????) ;;
        *)
            if [ "$LANG_UI" = "ru" ]; then
                warn "Формат ключа выглядит нестандартно — продолжаем"
            else
                warn "Key format looks unusual — continuing anyway"
            fi
            ;;
    esac
}

# ── Write config ──────────────────────────────────────────────────────────────
write_config() {
    mkdir -p "$CONFIG_DIR" "$STATE_DIR"

    OLD_HWID=""
    OLD_DEVICE_NAME=""
    OLD_CACHE_TTL="30m"
    OLD_LISTEN="0.0.0.0:16698"
    OLD_LOG_LEVEL="info"
    if [ -f "$CONFIG_FILE" ]; then
        OLD_HWID=$(grep '^SOTA_HWID=' "$CONFIG_FILE" 2>/dev/null | cut -d= -f2-)
        OLD_DEVICE_NAME=$(grep '^SOTA_DEVICE_NAME=' "$CONFIG_FILE" 2>/dev/null | cut -d= -f2-)
        OLD_CACHE_TTL=$(grep '^SOTA_CACHE_TTL=' "$CONFIG_FILE" 2>/dev/null | cut -d= -f2-)
        OLD_LISTEN=$(grep '^SOTA_LISTEN=' "$CONFIG_FILE" 2>/dev/null | cut -d= -f2-)
        OLD_LOG_LEVEL=$(grep '^SOTA_LOG_LEVEL=' "$CONFIG_FILE" 2>/dev/null | cut -d= -f2-)
        [ -z "$OLD_CACHE_TTL" ] && OLD_CACHE_TTL="30m"
        [ -z "$OLD_LISTEN" ]    && OLD_LISTEN="0.0.0.0:16698"
        [ -z "$OLD_LOG_LEVEL" ] && OLD_LOG_LEVEL="info"
    fi

    cat > "$CONFIG_FILE" << EOF
# sota-headless configuration
# Updated: $(date -Iseconds 2>/dev/null || date)

SOTA_ACCESS_KEY=${ACCESS_KEY}
SOTA_BASE_DIR=${CONFIG_DIR}
SOTA_LISTEN=${OLD_LISTEN}
SOTA_CACHE_TTL=${OLD_CACHE_TTL}
SOTA_LOG_LEVEL=${OLD_LOG_LEVEL}
EOF

    if [ -n "$OLD_HWID" ]; then
        echo "SOTA_HWID=${OLD_HWID}" >> "$CONFIG_FILE"
    fi
    if [ -n "$OLD_DEVICE_NAME" ]; then
        echo "SOTA_DEVICE_NAME=${OLD_DEVICE_NAME}" >> "$CONFIG_FILE"
    fi

    chmod 600 "$CONFIG_FILE"
    if [ "$LANG_UI" = "ru" ]; then
        ok "Конфигурация сохранена в $CONFIG_FILE"
    else
        ok "Config saved to $CONFIG_FILE"
    fi
}

# ── Main ──────────────────────────────────────────────────────────────────────
main() {
    check_root
    select_language

    check_deps
    detect_init
    ARCH=$(detect_arch)
    if [ "$SOTA_UPX" = "1" ] || [ "$SOTA_UPX" = "true" ]; then
        ARCH="${ARCH}-upx"
    fi
    if [ "$LANG_UI" = "ru" ]; then
        info "Архитектура: $ARCH (система: $INIT_SYSTEM, версия: $INSTALL_VERSION)"
    else
        info "Architecture: $ARCH (init: $INIT_SYSTEM, version: $INSTALL_VERSION)"
    fi

    check_existing

    ask_access_key

    if [ "$LANG_UI" = "ru" ]; then
        hdr "Загрузка файлов"
    else
        hdr "Downloading"
    fi

    if [ "$UPGRADING" = "1" ]; then
        if [ "$LANG_UI" = "ru" ]; then
            info "Остановка существующей службы..."
        else
            info "Stopping existing service..."
        fi
        if [ "$INIT_SYSTEM" = "systemd" ]; then
            systemctl stop sota-headless 2>/dev/null || true
        elif [ "$INIT_SYSTEM" = "procd" ] && [ -x "$INIT_DST" ]; then
            "$INIT_DST" stop 2>/dev/null || true
        fi
        sleep 1
    fi

    # Binary
    TMP_BIN="/tmp/sota-headless"
    download "${GITHUB_RELEASES}/sota-headless-${ARCH}" "$TMP_BIN"
    chmod +x "$TMP_BIN"
    mv "$TMP_BIN" "$BIN_DST"
    if [ "$LANG_UI" = "ru" ]; then
        ok "Бинарный файл → $BIN_DST"
    else
        ok "Binary → $BIN_DST"
    fi

    # Service config / daemon
    SCRIPT_DIR="$(dirname "$0" 2>/dev/null || true)"
    if [ "$INIT_SYSTEM" = "systemd" ]; then
        if [ -f "${SCRIPT_DIR}/sota-headless.service" ]; then
            cp "${SCRIPT_DIR}/sota-headless.service" "$SYSTEMD_SERVICE"
        else
            download "${GITHUB_RAW}/scripts/sota-headless.service" "$SYSTEMD_SERVICE"
        fi
        systemctl daemon-reload
        if [ "$LANG_UI" = "ru" ]; then
            ok "Служба systemd → $SYSTEMD_SERVICE"
        else
            ok "systemd service → $SYSTEMD_SERVICE"
        fi
    elif [ "$INIT_SYSTEM" = "procd" ]; then
        if [ -f "${SCRIPT_DIR}/sota-headless.init" ]; then
            cp "${SCRIPT_DIR}/sota-headless.init" "$INIT_DST"
        else
            download "${GITHUB_RAW}/scripts/sota-headless.init" "$INIT_DST"
        fi
        chmod +x "$INIT_DST"
        if [ "$LANG_UI" = "ru" ]; then
            ok "Скрипт службы → $INIT_DST"
        else
            ok "Init script → $INIT_DST"
        fi
    else
        if [ "$LANG_UI" = "ru" ]; then
            warn "Неизвестная система инициализации. Бинарник установлен в $BIN_DST, запускайте вручную."
        else
            warn "Unknown init system. Binary installed to $BIN_DST, please start manually."
        fi
    fi

    # Config
    write_config

    if [ "$PRESERVE_DEVICE" = "1" ]; then
        if [ "$LANG_UI" = "ru" ]; then
            ok "Идентификатор устройства сохранён ($DEVICE_JSON)"
        else
            ok "Device identity preserved ($DEVICE_JSON)"
        fi
    else
        if [ "$LANG_UI" = "ru" ]; then
            info "Идентификатор устройства будет сгенерирован при первом запуске"
        else
            info "Device identity will be generated on first start"
        fi
    fi

    if [ "$LANG_UI" = "ru" ]; then
        hdr "Запуск службы"
    else
        hdr "Starting service"
    fi

    if [ "$INIT_SYSTEM" = "systemd" ]; then
        systemctl enable sota-headless >/dev/null 2>&1
        systemctl restart sota-headless
    elif [ "$INIT_SYSTEM" = "procd" ]; then
        "$INIT_DST" enable
        "$INIT_DST" start
    fi
    sleep 2

    HEALTH_OK=0
    if command -v curl >/dev/null 2>&1; then
        curl -fsS -o /dev/null "http://127.0.0.1:16698/health" 2>/dev/null && HEALTH_OK=1
    elif command -v wget >/dev/null 2>&1; then
        wget -q -O /dev/null "http://127.0.0.1:16698/health" 2>/dev/null && HEALTH_OK=1
    fi

    if [ "$HEALTH_OK" = "1" ]; then
        if [ "$LANG_UI" = "ru" ]; then
            ok "Служба запущена и отвечает на запросы!"
        else
            ok "Service is up and responding!"
        fi
    else
        if [ "$LANG_UI" = "ru" ]; then
            warn "Служба запущена, но /health пока не ответил — возможно, идёт инициализация"
            if [ "$INIT_SYSTEM" = "systemd" ]; then
                info "Проверка: journalctl -u sota-headless -n 50"
            else
                info "Проверка: logread | grep sota"
            fi
        else
            warn "Service started but /health not responding yet — may still be initializing"
            if [ "$INIT_SYSTEM" = "systemd" ]; then
                info "Check: journalctl -u sota-headless -n 50"
            else
                info "Check: logread | grep sota"
            fi
        fi
    fi

    # Detect IP
    ROUTER_IP=""
    if command -v uci >/dev/null 2>&1; then
        ROUTER_IP=$(uci -q get network.lan.ipaddr 2>/dev/null | head -n1)
    fi
    if [ -z "$ROUTER_IP" ] && command -v ip >/dev/null 2>&1; then
        ROUTER_IP=$(ip -4 addr show br-lan 2>/dev/null | awk '/inet /{print $2}' | head -n1)
        [ -z "$ROUTER_IP" ] && ROUTER_IP=$(ip -4 addr show lan 2>/dev/null | awk '/inet /{print $2}' | head -n1)
        [ -z "$ROUTER_IP" ] && ROUTER_IP=$(ip route get 1 2>/dev/null | awk '{print $7; exit}')
    fi
    if [ -z "$ROUTER_IP" ] && command -v hostname >/dev/null 2>&1; then
        ROUTER_IP=$(hostname -I 2>/dev/null | awk '{print $1}')
    fi
    ROUTER_IP="${ROUTER_IP%% *}"
    ROUTER_IP="${ROUTER_IP%%/*}"
    ROUTER_IP=$(echo "$ROUTER_IP" | tr -d ' \r\n')
    [ -z "$ROUTER_IP" ] && ROUTER_IP="127.0.0.1"

    printf "\n%b════════════════════════════════════════════%b\n" "$YEL" "$NC"
    if [ "$LANG_UI" = "ru" ]; then
        printf "%b    Готово!%b\n" "$YEL" "$NC"
        printf "%b════════════════════════════════════════════%b\n\n" "$YEL" "$NC"
        printf "%b  Ссылки на подписки:%b\n\n" "$GRY" "$NC"
        printf "  %bMihomo / Clash / Zashboard%b\n" "$YEL" "$NC"
        printf "  %bhttp://${ROUTER_IP}:16698/sub/mihomo%b\n\n" "$DIM" "$NC"
        printf "  %bSing-box outbounds%b\n" "$YEL" "$NC"
        printf "  %bhttp://${ROUTER_IP}:16698/sub/singbox%b\n\n" "$DIM" "$NC"
        printf "  %bBase64 (v2rayN, Nekoray, v2rayNG)%b\n" "$YEL" "$NC"
        printf "  %bhttp://${ROUTER_IP}:16698/sub/base64%b\n\n" "$DIM" "$NC"
        printf "  %bПрямые ссылки vless://%b\n" "$YEL" "$NC"
        printf "  %bhttp://${ROUTER_IP}:16698/sub/vless%b\n\n" "$DIM" "$NC"
        printf "%b  Проверка: %bcurl http://127.0.0.1:16698/health%b\n" "$GRY" "$DIM" "$NC"
        if [ "$INIT_SYSTEM" = "systemd" ]; then
            printf "%b  Статус:   %bsystemctl status sota-headless%b\n" "$GRY" "$DIM" "$NC"
            printf "%b  Логи:     %bjournalctl -u sota-headless -f%b\n\n" "$GRY" "$DIM" "$NC"
        else
            printf "%b  Статус:   %bservice sota-headless status%b\n" "$GRY" "$DIM" "$NC"
            printf "%b  Логи:     %blogread -f -e sota%b\n\n" "$GRY" "$DIM" "$NC"
        fi
    else
        printf "%b    Done!%b\n" "$YEL" "$NC"
        printf "%b════════════════════════════════════════════%b\n\n" "$YEL" "$NC"
        printf "%b  Subscription URLs:%b\n\n" "$GRY" "$NC"
        printf "  %bMihomo / Clash / Zashboard%b\n" "$YEL" "$NC"
        printf "  %bhttp://${ROUTER_IP}:16698/sub/mihomo%b\n\n" "$DIM" "$NC"
        printf "  %bSing-box outbounds%b\n" "$YEL" "$NC"
        printf "  %bhttp://${ROUTER_IP}:16698/sub/singbox%b\n\n" "$DIM" "$NC"
        printf "  %bBase64 (v2rayN, Nekoray, v2rayNG)%b\n" "$YEL" "$NC"
        printf "  %bhttp://${ROUTER_IP}:16698/sub/base64%b\n\n" "$DIM" "$NC"
        printf "  %bPlain vless:// links%b\n" "$YEL" "$NC"
        printf "  %bhttp://${ROUTER_IP}:16698/sub/vless%b\n\n" "$DIM" "$NC"
        printf "%b  Health:  %bcurl http://127.0.0.1:16698/health%b\n" "$GRY" "$DIM" "$NC"
        if [ "$INIT_SYSTEM" = "systemd" ]; then
            printf "%b  Status:  %bsystemctl status sota-headless%b\n" "$GRY" "$DIM" "$NC"
            printf "%b  Logs:    %bjournalctl -u sota-headless -f%b\n\n" "$GRY" "$DIM" "$NC"
        else
            printf "%b  Status:  %bservice sota-headless status%b\n" "$GRY" "$DIM" "$NC"
            printf "%b  Logs:    %blogread -f -e sota%b\n\n" "$GRY" "$DIM" "$NC"
        fi
    fi
}

main "$@"
