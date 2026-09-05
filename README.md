<div align="center">

# sota-headless

**Сервис генерации подписок Sota Connect**

[![Go](https://img.shields.io/badge/Go-1.26-00ADD8?style=flat&logo=go&logoColor=white)](https://go.dev)
[![Release](https://img.shields.io/github/v/release/paintingpromisesss/sota_headless?style=flat&color=yellow)](https://github.com/paintingpromisesss/sota_headless/releases/latest)
[![License](https://img.shields.io/badge/license-MIT-grey?style=flat)](LICENSE)
[![OpenWrt](https://img.shields.io/badge/OpenWrt-25.x-00B5E2?style=flat&logo=openwrt&logoColor=white)](https://openwrt.org)

[English](README_EN.md) • **Русский**

Локальный сервис для получения параметров узлов Sota Connect через API и их предоставления в виде стандартных подписок для сторонних клиентов.

</div>

---

## Схема работы

```
Клиент (Mihomo, v2rayNG, Happ, sing-box и др.)
        │
        │  GET http://<host>:16698/sub/<формат>
        ▼
  sota-headless (локальный сервис)
        │
        │  Запросы к API (X-Access-Key, X-HWID)
        ▼
  API Sota Connect  →  параметры узлов VLESS + Reality
```

Сервис не маршрутизирует трафик. Он запрашивает данные узлов через API, формирует подписку в выбранном формате и сохраняет результат в кэше оперативной памяти (по умолчанию на 30 минут).

---

## Установка

### Стандартная установка

**Windows (PowerShell от имени администратора):**
```powershell
irm https://raw.githubusercontent.com/paintingpromisesss/sota_headless/main/scripts/install.ps1 | iex
```

**Linux (Ubuntu, Debian, CentOS, Arch, серверы):**
```sh
curl -fsSL https://raw.githubusercontent.com/paintingpromisesss/sota_headless/main/scripts/install.sh | sudo sh
```

**OpenWrt (роутеры):**
```sh
wget -O /tmp/install.sh https://raw.githubusercontent.com/paintingpromisesss/sota_headless/main/scripts/install.sh && sh /tmp/install.sh
```

### Сборка с UPX (для устройств с ограниченной флеш-памятью)

Для роутеров с малым объемом системной памяти доступна сборка, сжатая UPX:

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

> #### Сравнение вариантов сборки
>
> | Параметр | Стандартная сборка | Сборка UPX | Примечание |
> |---|---|---|---|
> | Размер на накопителе | ~6.0–6.7 МБ | ~1.7–2.0 МБ | Экономия ~70% дискового пространства |
> | Потребление RAM (RSS) | ~10–14 МБ | ~16–22 МБ | Дополнительные ~5–7 МБ RAM при работе |
> | Время запуска | ~5 мс | ~30–60 мс | Время на распаковку исполняемого файла в память |

### Поддерживаемые платформы роутеров

| Платформа / SoC | Архитектура | Примеры устройств |
|-----------------|-------------|-------------------|
| MediaTek Filogic (MT7981/MT7986) | `arm64` | Netis NX31, GL.iNet MT3000 |
| Rockchip RK3568 | `arm64` | NanoPi R5S, R5C |
| MediaTek MT7621 | `mipsle` | Xiaomi R3G, Keenetic Giga |
| Atheros AR9xxx | `mips` | TP-Link WR1043 |
| x86-64 | `amd64` | PC Engines APU, x86-роутеры, мини-ПК |

---

## Форматы подписок

После запуска сервиса ссылки указываются в настройках клиента:

| Формат | URL | Совместимые клиенты |
|--------|-----|---------------------|
| **Mihomo YAML** | `http://<host>:16698/sub/mihomo` | Mihomo, Clash.Meta, Zashboard |
| **Base64** | `http://<host>:16698/sub/base64` | Happ, v2rayNG, Shadowrocket, Streisand |
| **Построчный VLESS** | `http://<host>:16698/sub/vless` | Прямой импорт ссылок вида `vless://` |
| **sing-box JSON** | `http://<host>:16698/sub/singbox` | Массив `outbounds` для конфигураций sing-box |

> `<host>` — IP-адрес устройства в локальной сети или `127.0.0.1`, если клиент запущен на том же устройстве.

---

## Конфигурация

Параметры задаются в файле конфигурации:
- **Windows**: `C:\Program Files\sota-headless\sota-headless.env`
- **Linux / OpenWrt**: `/etc/sota-headless/sota-headless.env`

```env
SOTA_ACCESS_KEY=ваш-ключ-доступа
SOTA_LISTEN=0.0.0.0:16698
SOTA_CACHE_TTL=30m
# Уровень логирования: debug, info (по умолчанию), warn, error
SOTA_LOG_LEVEL=info
```

Применение изменений конфигурации требует перезапуска службы:

- **Windows (PowerShell)**: `Restart-Service sota-headless`
- **Linux**: `systemctl restart sota-headless`
- **OpenWrt**: `service sota-headless restart`

Принудительное обновление кэша без перезапуска:

```sh
curl -X POST http://127.0.0.1:16698/sub/refresh
```

---

## Обновление

Для обновления достаточно повторно выполнить команду установки. Скрипт заменит исполняемый файл на актуальный, сохранив существующую конфигурацию и файл идентификации устройства (`device.json`).

---

## Управление и диагностика

```sh
# Состояние и управление службой:
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

# Проверка доступности сервиса:
curl http://127.0.0.1:16698/health

# Статус сервиса и параметры кэша:
curl http://127.0.0.1:16698/status

# Сведения о подписке (срок действия, ограничения):
curl http://127.0.0.1:16698/profile

# Список доступных локаций:
curl http://127.0.0.1:16698/locations
```

---

## Подробное описание работы

Сервис представляет собой локальный HTTP-сервер, транслирующий данные из API Sota Connect в форматы стандартных подписок.

### Состав локального API

#### Подписки

| Метод и путь | Формат ответа | Назначение |
|---|---|---|
| `GET /sub`<br>`GET /sub/mihomo` | YAML (`text/yaml`) | Подписка в формате `proxy-provider` для Mihomo и Clash.Meta |
| `GET /sub/vless` | Текст (`text/plain`) | Построчный список ссылок `vless://` |
| `GET /sub/base64` | Base64 (`text/plain`) | Список ссылок `vless://`, закодированный в Base64 |
| `GET /sub/singbox` | JSON (`application/json`) | JSON с массивом `outbounds` для конфигурации sing-box |

#### Информация и статус

| Метод и путь | Формат ответа | Назначение |
|---|---|---|
| `GET /`<br>`GET /health` | JSON (`application/json`) | Проверка доступности сервиса, версия и базовая сводка |
| `GET /status` | JSON (`application/json`) | Состояние сервиса: количество нод в кэше, возраст кэша, TTL, активный API base, ошибки |
| `GET /profile` | JSON (`application/json`) | Сведения о подписке из API Sota (активность, сроки, ограничения) |
| `GET /locations` | JSON (`application/json`) | Список доступных локаций и серверов из API Sota |
| `GET /device` | JSON (`application/json`) | Идентификационные данные (`x_device_name` и маскированный `x_hwid`) |

#### Управление

| Метод и путь | Формат ответа | Назначение |
|---|---|---|
| `POST /sub/refresh` | JSON (`application/json`) | Сброс локального кэша и принудительный запрос актуальных нод из API Sota |

---

### 1. Инициализация и идентификация устройства

При первом запуске сервис регистрирует локальный профиль устройства:
- В рабочей директории (`state/device.json`) создается файл с параметрами `x_hwid` и `x_device_name`.
- Значение `x_hwid` генерируется как хеш SHA-256 от имени хоста, ОС, архитектуры и 32 случайных байт.
- Значение `x_device_name` по умолчанию формируется по шаблону `<hostname>_<os>_<arch>`.
- При необходимости параметры `HWID` и `DeviceName` можно переопределить через переменные `SOTA_HWID` и `SOTA_DEVICE_NAME`.

### 2. Взаимодействие с API Sota Connect

Запросы к API отправляются с набором заголовков:
- `X-Access-Key` — ключ доступа пользователя;
- `X-HWID` — идентификатор устройства;
- `X-Device-Name` — имя устройства;
- `User-Agent` — идентификатор клиента (по умолчанию эмулируется официальное приложение).

### 3. Сбор и обработка конфигураций узлов

Получение списка серверов выполняется в два этапа:
1. **Запрос списка локаций**: метод `GET /public/connection/list` возвращает доступные шлюзы с их `gate_id`, названиями и кодами стран.
2. **Опрос параметров подключения**: для каждого `gate_id` выполняется отдельный запрос `GET /public/connection/connect?gate_id=<id>`.

Из ответа каждого шлюза извлекаются параметры подключения VLESS + Reality:
- Адрес сервера (`server`) и порт (`server_port`);
- Идентификатор пользователя (`uuid`);
- Параметры Reality: публичный ключ (`public_key`), короткий идентификатор (`short_id`), SNI (`server_name`), отпечаток TLS (`fingerprint`) и режим потока (`flow`).

Собранные узлы приводятся к единой внутренней структуре `Node` и сортируются по `gate_id`.

### 4. Кэширование

- Полученный массив узлов сохраняется в оперативной памяти.
- Кэш нужен, чтобы не запрашивать узлы заново при каждом обращении клиента и не создавать лишнюю нагрузку на API.
- Время жизни кэша регулируется параметром `SOTA_CACHE_TTL` (по умолчанию 30 минут). По истечении TTL следующий запрос клиента инициирует обновление данных из API.
- Сбросить кэш и принудительно запросить свежие ноды можно через `POST /sub/refresh`.

### 5. Системные ресурсы

- Сервис только формирует подписки и не участвует в передаче сетевого трафика.
- Потребление памяти ограничено на уровне рантайма (48 МБ), чтобы процесс не завершался аварийно по OOM на слабых роутерах.

---

<div align="center">
<sub>Не связано с Cat Connect Oy / Sota Connect. Проект предоставляется «как есть».</sub>
</div>
