<div align="center">

# sota-headless

**Сервис генерации подписок Sota Connect для роутеров OpenWrt**

[![Go](https://img.shields.io/badge/Go-1.26-00ADD8?style=flat&logo=go&logoColor=white)](https://go.dev)
[![Release](https://img.shields.io/github/v/release/paintingpromisesss/sota_headless?style=flat&color=yellow)](https://github.com/paintingpromisesss/sota_headless/releases/latest)
[![License](https://img.shields.io/badge/license-MIT-grey?style=flat)](LICENSE)
[![OpenWrt](https://img.shields.io/badge/OpenWrt-25.x-00B5E2?style=flat&logo=openwrt&logoColor=white)](https://openwrt.org)

[English](README_EN.md) • **Русский**

Sota Connect закрывает свои подписки внутри проприетарного приложения.  
Этот модуль авторизуется в API сервиса и отдаёт узлы в виде стандартной подписки — прямо с вашего роутера.

</div>

---

## Как это работает

```
Ваше устройство (Happ / Mihomo / v2rayNG / Zashboard / ...)
        │
        │  GET http://router:16698/sub/mihomo
        ▼
  sota-headless  (фоновый сервис на роутере)
        │
        │  X-Access-Key + X-HWID → Sota API
        ▼
   meowconnect.com  →  VLESS + Reality узлы
```

Ноды кэшируются на 30 минут. Никаких внешних зависимостей, никакого встроенного sing-box, бинарник всего ~7 МБ.

---

## Установка

### Быстрая установка

**Для Windows (PowerShell от имени Администратора):**
```powershell
irm https://raw.githubusercontent.com/paintingpromisesss/sota_headless/main/scripts/install.ps1 | iex
```

**Для Linux (Ubuntu / Debian / CentOS / Arch / VPS / серверы):**
```sh
curl -fsSL https://raw.githubusercontent.com/paintingpromisesss/sota_headless/main/scripts/install.sh | sudo sh
```

**Для OpenWrt (роутеры):**
```sh
wget -O /tmp/install.sh https://raw.githubusercontent.com/paintingpromisesss/sota_headless/main/scripts/install.sh && sh /tmp/install.sh
```

Инсталлятор сделает всё автоматически:
- определит платформу (Windows, Linux с `systemd` или OpenWrt с `procd`) и архитектуру процессора
- запросит ваш ключ доступа (Access Key)
- скачает бинарник и настроит системную службу (на Windows — нативную службу Windows и правило брандмауэра)
- запустит фоновый сервис и выведет ссылки на подписки

### Поддерживаемые роутеры

| Чип / Платформа | Архитектура | Примеры устройств |
|-----------------|-------------|-------------------|
| MediaTek Filogic (MT7981/MT7986) | `arm64` | Netis NX31, GL.iNet MT3000 |
| Rockchip RK3568 | `arm64` | NanoPi R5S, R5C |
| MediaTek MT7621 | `mipsle` | Xiaomi R3G, Keenetic Giga |
| Atheros AR9xxx | `mips` | TP-Link WR1043 |
| x86/x64 | `amd64` | PC Engines APU, Мини-ПК |

---

## Ссылки на подписки

После установки укажите подходящий адрес в вашем клиенте:

| Формат | URL | Где использовать |
|--------|-----|------------------|
| **Mihomo YAML** | `http://router-ip:16698/sub/mihomo` | Mihomo, Zashboard, Clash.Meta |
| **Base64** | `http://router-ip:16698/sub/base64` | Happ, v2rayNG, Shadowrocket |
| **Обычные ссылки vless://** | `http://router-ip:16698/sub/vless` | Ручной импорт |
| **sing-box JSON** | `http://router-ip:16698/sub/singbox` | Массив outbounds для sing-box |

> Замените `router-ip` на локальный IP-адрес вашего роутера (например, `192.168.0.1` или `192.168.1.1`).

---

## Настройка
 
Файл конфигурации хранится по пути:
- **Windows**: `C:\Program Files\sota-headless\sota-headless.env`
- **Linux / OpenWrt**: `/etc/sota-headless/sota-headless.env`

```env
SOTA_ACCESS_KEY=ваш-ключ-доступа
SOTA_LISTEN=0.0.0.0:16698
SOTA_CACHE_TTL=30m
# Уровень логирования: debug, info (по умолчанию), warn, error
SOTA_LOG_LEVEL=info
```

После изменения конфигурации перезапустите сервис:

- **Windows (PowerShell)**: `Restart-Service sota-headless`
- **Linux**: `systemctl restart sota-headless`
- **OpenWrt**: `service sota-headless restart`

Для принудительного обновления кэша нод без перезапуска:

```sh
curl -X POST http://127.0.0.1:16698/sub/refresh
```

---

## Обновление

Просто запустите инсталлятор заново — он автоматически обнаружит установленную версию, сохранит ваш ключ доступа и идентификатор устройства, и обновит бинарник.

---

## Полезные команды

```sh
# Управление службой:
# Windows (PowerShell от имени Администратора):
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

# Проверка работоспособности
curl http://127.0.0.1:16698/health

# Информация о подписке (срок, лимиты)
curl http://127.0.0.1:16698/profile

# Список доступных локаций
curl http://127.0.0.1:16698/locations
```

---

<div align="center">
<sub>Не связано с Cat Connect Oy / Sota Connect. Используйте на своё усмотрение.</sub>
</div>
