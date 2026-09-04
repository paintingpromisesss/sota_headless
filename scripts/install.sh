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

# ── Language selection ────────────────────────────────────────────────────────
select_language() {
    printf "\n%b════════════════════════════════════════════%b\n" "$YEL" "$NC"
    printf "%b    sota-headless  ·  OpenWrt installer%b\n" "$YEL" "$NC"
    printf "%b════════════════════════════════════════════%b\n\n" "$YEL" "$NC"
    printf "%b  [1] English (default)\n  [2] Русский%b\n\n" "$GRY" "$NC"
    ask "Select language / Выберите язык [1/2]:"
    read LANG_CHOICE

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
        aarch64)        echo "linux-arm64"  ;;
        armv7l|armv6l)  echo "linux-arm"    ;;
        x86_64)         echo "linux-amd64"  ;;
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
    for cmd in wget chmod; do
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
    wget -q -O "$DST" "$URL" || {
        if [ "$LANG_UI" = "ru" ]; then
            error "Ошибка загрузки: $URL"
        else
            error "Download failed: $URL"
        fi
    }
}

# ── Check if service already running ─────────────────────────────────────────
check_existing() {
    UPGRADING=0
    if [ -x "$INIT_DST" ]; then
        if "$INIT_DST" status >/dev/null 2>&1; then
            if [ "$LANG_UI" = "ru" ]; then
                warn "Служба sota-headless уже запущена — выполняется обновление"
            else
                warn "sota-headless is already running — this is an upgrade"
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
        read INPUT_KEY
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
        read INPUT_KEY
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
    select_language

    check_deps
    ARCH=$(detect_arch)
    if [ "$LANG_UI" = "ru" ]; then
        info "Архитектура: $ARCH"
    else
        info "Architecture: $ARCH"
    fi

    check_existing

    ask_access_key

    if [ "$LANG_UI" = "ru" ]; then
        hdr "Загрузка файлов"
    else
        hdr "Downloading"
    fi

    if [ "$UPGRADING" = "1" ] && [ -x "$INIT_DST" ]; then
        if [ "$LANG_UI" = "ru" ]; then
            info "Остановка существующей службы..."
        else
            info "Stopping existing service..."
        fi
        "$INIT_DST" stop 2>/dev/null || true
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

    # Init script
    download "${GITHUB_RAW}/scripts/sota-headless.init" "$INIT_DST"
    chmod +x "$INIT_DST"
    if [ "$LANG_UI" = "ru" ]; then
        ok "Скрипт службы → $INIT_DST"
    else
        ok "Init script → $INIT_DST"
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
    "$INIT_DST" enable
    "$INIT_DST" start
    sleep 2

    if wget -q -O /dev/null "http://127.0.0.1:16698/health" 2>/dev/null; then
        if [ "$LANG_UI" = "ru" ]; then
            ok "Служба запущена и отвечает на запросы!"
        else
            ok "Service is up and responding!"
        fi
    else
        if [ "$LANG_UI" = "ru" ]; then
            warn "Служба запущена, но /health пока не ответил — возможно, идёт инициализация"
            info "Проверка: logread | grep sota"
        else
            warn "Service started but /health not responding yet — may still be initializing"
            info "Check: logread | grep sota"
        fi
    fi

    # Detect LAN IP on OpenWrt (br-lan or uci network.lan.ipaddr)
    ROUTER_IP=$(uci -q get network.lan.ipaddr 2>/dev/null | head -n1)
    [ -z "$ROUTER_IP" ] && ROUTER_IP=$(ip -4 addr show br-lan 2>/dev/null | awk '/inet /{print $2}' | head -n1)
    [ -z "$ROUTER_IP" ] && ROUTER_IP=$(ip -4 addr show lan 2>/dev/null | awk '/inet /{print $2}' | head -n1)
    [ -z "$ROUTER_IP" ] && ROUTER_IP=$(ip route get 1 2>/dev/null | awk '{print $7; exit}')
    # Strip CIDR netmask and extra fields if present (e.g. 192.168.0.1/24 -> 192.168.0.1)
    ROUTER_IP="${ROUTER_IP%% *}"
    ROUTER_IP="${ROUTER_IP%%/*}"
    ROUTER_IP=$(echo "$ROUTER_IP" | tr -d ' \r\n')
    [ -z "$ROUTER_IP" ] && ROUTER_IP="192.168.0.1"

    printf "\n%b════════════════════════════════════════════%b\n" "$YEL" "$NC"
    if [ "$LANG_UI" = "ru" ]; then
        printf "%b    Готово!%b\n" "$YEL" "$NC"
        printf "%b════════════════════════════════════════════%b\n\n" "$YEL" "$NC"
        printf "%b  Ссылки на подписки:%b\n\n" "$GRY" "$NC"
        printf "  %bMihomo / Zashboard%b\n" "$YEL" "$NC"
        printf "  %bhttp://${ROUTER_IP}:16698/sub/mihomo%b\n\n" "$DIM" "$NC"
        printf "  %bBase64 (универсальный)%b\n" "$YEL" "$NC"
        printf "  %bhttp://${ROUTER_IP}:16698/sub/base64%b\n\n" "$DIM" "$NC"
        printf "  %bПрямые ссылки vless://%b\n" "$YEL" "$NC"
        printf "  %bhttp://${ROUTER_IP}:16698/sub/vless%b\n\n" "$DIM" "$NC"
        printf "%b  Проверка: %bcurl http://127.0.0.1:16698/health%b\n" "$GRY" "$DIM" "$NC"
        printf "%b  Логи:     %blogread | grep sota%b\n\n" "$GRY" "$DIM" "$NC"
    else
        printf "%b    Done!%b\n" "$YEL" "$NC"
        printf "%b════════════════════════════════════════════%b\n\n" "$YEL" "$NC"
        printf "%b  Subscription URLs:%b\n\n" "$GRY" "$NC"
        printf "  %bMihomo / Zashboard%b\n" "$YEL" "$NC"
        printf "  %bhttp://${ROUTER_IP}:16698/sub/mihomo%b\n\n" "$DIM" "$NC"
        printf "  %bBase64 (universal)%b\n" "$YEL" "$NC"
        printf "  %bhttp://${ROUTER_IP}:16698/sub/base64%b\n\n" "$DIM" "$NC"
        printf "  %bPlain vless:// links%b\n" "$YEL" "$NC"
        printf "  %bhttp://${ROUTER_IP}:16698/sub/vless%b\n\n" "$DIM" "$NC"
        printf "%b  Health:  %bcurl http://127.0.0.1:16698/health%b\n" "$GRY" "$DIM" "$NC"
        printf "%b  Logs:    %blogread | grep sota%b\n\n" "$GRY" "$DIM" "$NC"
    fi
}

main "$@"
