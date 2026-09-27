# 🐘 Wunschpunsch: Database cluster TUI
<img width="1024" height="559" alt="image" src="https://github.com/user-attachments/assets/535a4bae-0064-4a7a-8d1d-92f547d62edd" />

[English](#english) | [Русский](#русский)

---

## English

### Overview

**Cluster TUI** is a terminal-based management console written in Go. It provides real-time visibility and monitoring for high-availability **PostgreSQL** clusters orchestrated by **Patroni**.

Built on the modern [Charm](https://charm.sh/) ecosystem (`bubbletea`, `lipgloss`, `bubbles`), it delivers a responsive, keyboard-driven interface with zero external GUI dependencies.

### Key Features

* **Auto-Discovery:** Connects to a single Patroni node and automatically discovers all cluster members.
* **Topology View:** Displays roles (`Leader`, `Sync Standby`, `Replica`), timelines, states, and replication lag (MB).
* **PG Performance:** Direct queries via `pgx/v5` for active connections (`max_connections`) and Cache Hit Ratio.
* **High Availability:** Built-in fallback routing that cycles through Patroni endpoints if a node fails.
* **Modern TUI:** Tabbed navigation, clean styling, and full keyboard control.

### Tech Stack

* **Language:** Go `1.25.0`
* **TUI Frameworks:**
  * [`charmbracelet/bubbletea`](https://github.com/charmbracelet/bubbletea) — Elm-inspired architecture
  * [`charmbracelet/lipgloss`](https://github.com/charmbracelet/lipgloss) — Styling and layouts
  * [`charmbracelet/bubbles`](https://github.com/charmbracelet/bubbles) — UI components (tables, text inputs)
* **Database Driver:** [`jackc/pgx/v5`](https://github.com/jackc/pgx) — PostgreSQL connection pooling

### Prerequisites

* **Go:** `1.25` or higher
* **Patroni REST API:** Reachable endpoint (default: `8008` / `8001`)
* **PostgreSQL:** Access on port `5432` with privileges to read `pg_stat_activity` and `pg_statio_user_tables`

### Quick Start

```bash
# Clone repository
git clone https://github.com/your-username/cluster-tui.git
cd wunschpunsch

# Download dependencies
go mod download

# Run directly
go run main.go

# Or compile binary
go build -o wunschpunsch main.go
./wunschpunsch
```

### Keybindings

**Connection Screen (`0: Connect`)**
* `Tab` / `Up` / `Down` — Navigate between input fields
* `Enter` — Submit and connect to cluster

**Main Dashboard (Tabs 1–4)**
* `1` .. `4` — Jump directly to tab (`1: Topology`, `2: Metrics`, `3: Actions`, `4: Event Logs`)
* `Tab` / `l` / `Right` — Next tab
* `Shift+Tab` / `h` / `Left` — Previous tab
* `Ctrl+C` / `q` — Exit program

### Roadmap

- [x] Patroni REST API integration & auto-discovery
- [x] Periodic state updates (every 2 seconds)
- [x] Cluster topology table (Lag, Timeline, Roles)
- [x] Basic PostgreSQL metrics (Cache Hit Ratio, Connection Pools)
- [x] **Cluster Management Actions:**
  - [x] Graceful Switchover / Force Failover
  - [x] Reinitialize Replica
  - [x] Maintenance Mode toggle (Pause/Resume)
- [x] Real-time Patroni log streaming
  UI Enhancements & Application Settings
- [ ] **Global Settings Menu (Modal / Settings Tab):**
  - [ ] On-the-fly UI Language Switching (English / Russian) via i18n.
  - [ ] Color Scheme Selection (Catppuccin, Nord, Monokai, High Contrast).
  - [ ] Configurable Cluster Polling Interval (Default: 2s).
- [ ] **Data Export & Logging:**
  - [ ] Local Metrics Snapshot Export (JSON / CSV).
  - [ ] Event Log Dump & Auto-saving to local file (`~/.wunschpunsch/logs/app.log`).
  Real-Time Event Logs Improvements
- [ ] **Sub-tab Filtering inside the Logs Viewport:**
  - [ ] `[All]` — Unified real-time event stream.
  - [ ] `[Postgres]` — Database-specific metrics, warnings, and errors.
  - [ ] `[HA Engine / Patroni]` — Role switches, timeline changes, and failover health checks.
  - [ ] `[System / Host]` — System alerts (high CPU/RAM/Disk utilization).
- [ ] **Interactive Log Features:**
  - [ ] In-stream Keyword Search & Filtering (via `/` hotkey).
  - [ ] Pause Auto-scroll on scroll-up.
  Advanced PostgreSQL Diagnostics
- [ ] **Long Query & Lock Monitoring (Active Queries):**
  - View running SQL queries directly from `pg_stat_activity`.
  - Highlight hung transactions (`idle in transaction`) with the ability to safely cancel/terminate long-running backends (`pg_cancel_backend` / `pg_terminate_backend`) directly from the TUI.
- [ ] **Bloat & Autovacuum Tracker:**
  - Monitor active `autovacuum` workers and table/index bloat ratios.
    Extended HA Stack Support
- [ ] **Corosync + Pacemaker Full Integration:**
  - Auto-detection and resource status visualization for `crm_mon` / `pcs status`.
- [ ] **PgBouncer / HAProxy Monitoring:**
  - Dedicated widget for connection pool status (active client/server pools, waiting clients).
    Profile Management & Production Safeguards
- [ ] **Connection Manager:**
  - Save and manage multiple cluster environments (Staging, Prod-1, Prod-2) in an encrypted local configuration file for quick switching.
- [ ] **Production Safeguard Mode:**
  - Visual high-visibility alerts (e.g., prominent red borders or mandatory cluster name confirmation) when performing destructive/critical actions (Failover / Reinit) on Production instances.
### License

Distributed under the **MIT License**.

---

## Русский

### Обзор проекта

**Wunschpunsch: Database cluster TUI** — это консольная утилита на языке Go, предназначенная для отслеживания состояния и управления высокодоступными кластерами **PostgreSQL** под управлением **Patroni**.

Проект разработан с использованием библиотек [Charm](https://charm.sh/) (`bubbletea`, `lipgloss`, `bubbles`), обеспечивающих удобный графический интерфейс прямо в терминале с поддержкой горячих клавиш.

### Основные возможности

* **Автообнаружение:** Достаточно указать одну ноду Patroni, чтобы автоматически найти остальные узлы кластера.
* **Топология:** Отображение ролей (`Leader`, `Sync Standby`, `Replica`), статусов, Timeline и отставания (MB).
* **Метрики PG:** Прямое подключение через `pgx/v5` для сбора активности соединений и Cache Hit Ratio.
* **Отказоустойчивость:** Автоматическое переключение на доступные эндпоинты Patroni при сбое узла.
* **Удобный TUI:** Понятное разделение по вкладкам, цветовые акценты и полная навигация с клавиатуры.

### Технологический стек

* **Язык программирования:** Go `1.25.0`
* **TUI Компоненты:**
  * [`charmbracelet/bubbletea`](https://github.com/charmbracelet/bubbletea) — архитектура приложения
  * [`charmbracelet/lipgloss`](https://github.com/charmbracelet/lipgloss) — стилизация элементов интерфейса
  * [`charmbracelet/bubbles`](https://github.com/charmbracelet/bubbles) — таблицы, текстовые поля
* **Драйвер базы данных:** [`jackc/pgx/v5`](https://github.com/jackc/pgx) — управление пулом соединений PostgreSQL

### Предварительные требования

* **Go:** Версия `1.25` или выше
* **Patroni REST API:** Доступ к порту `8008` (или `8001`)
* **PostgreSQL:** Доступ к порту `5432` с правом чтения системных представлений `pg_stat_activity` и `pg_statio_user_tables`

### Быстрый запуск

```bash
# Клонирование репозитория
git clone https://github.com/your-username/wunschpunsch.git
cd wunschpunsch

# Установка зависимостей
go mod download

# Прямой запуск
go run main.go

# Сборка бинарного файла
go build -o wunschpunsch main.go
./wunschpunsch
```

### Горячие клавиши

**Форма подключения (`0: Connect`)**
* `Tab` / `Up` / `Down` — Переход между полями ввода
* `Enter` — Подключиться к кластеру

**Основные вкладки (1–4)**
* `1` .. `4` — Быстрый переход по вкладкам (`1: Топология`, `2: Метрики`, `3: Действия`, `4: Логи`)
* `Tab` / `l` / `Right` — Следующая вкладка
* `Shift+Tab` / `h` / `Left` — Предыдущая вкладка
* `Ctrl+C` / `q` — Выход из приложения

### План разработки (Roadmap)

- [x] Авторизация и подключение к Patroni + PostgreSQL
- [x] Фоновое обновление состояния кластера каждые 2 секунды
- [x] Таблица топологии узлов (Lag, Timeline, Roles)
- [x] Сбор метрик PostgreSQL (Cache Hit Ratio, соединения)
- [ ] **Управление кластером из TUI:**
  - [x] Плановый (Switchover) и принудительный (Failover) смен лидера
  - [x] Реинициализация реплики (Reinitialize)
  - [x] Переключение режима обслуживания (Pause/Resume)
- [x] Просмотр потока логов событий Patroni в реальном времени
Улучшения UI & Настройки (App Settings)
[ ] Экран глобальных настроек (Settings Modal/Tab):
 - [ ] Переключение языка интерфейса (RU / EN) «на лету» (i18n).
 - [ ] Выбор цветовой схемы (Catppuccin, Nord, Monokai, High Contrast).
 - [ ] Настройка интервала опрашивания кластера (default: 2s).
Экспорт данных & Логирование:
 - [ ] Сохранение текущего снимка метрик в JSON / CSV на локальную машину.
 - [ ] Дамп и автосохранение истории событий логов в файл (~/.cluster-tui/logs/app.log).
[ ] Вкладки фильтрации (Sub-tabs) внутри Viewport логов:
 - [ ] [All] — единый поток всех событий.
 - [ ] [HA Engine / Patroni] — события смены ролей, таймлайнов, healthchecks.
 - [ ] [System / Host] — системные предупреждения (высокая загрузка CPU/RAM/Disk).
 [ ] Интерактивные возможности логов:
  - [ ] Текстовый поиск / фильтрация по ключевым словам по клавише /.
  - [ ] Пауза автоскролла логов при прокрутке вверх.
 [ ] Мониторинг долгих запросов и блокировок (Active Queries):
  - [ ] Просмотр выполняющихся в данный момент SQL-запросов из pg_stat_activity.
  - [ ] Подсветка зависших транзакций (idle in transaction) и возможность завершить долгий запрос прямо из TUI (pg_cancel_backend / pg_terminate_backend).
 [ ] Анализ Bloat и очистки (Autovacuum Tracker):
  - [ ] Отображение текущих процессов autovacuum и уровня раздувания таблиц/индексов.
 [ ] Полная поддержка Corosync + Pacemaker:
  - [ ] Детект и отображение ресурсов crm_mon / pcs status для Pacemaker-кластеров.
 [ ] Мониторинг PgBouncer / HAProxy:
  - [ ] Отдельный виджет состояния пулера соединений (количество активных клиентских и серверных пулов, waiting clients).
 [ ] Менеджер профилей (Connection Manager):
  - [ ] Возможность сохранять несколько конфигураций кластеров (Staging, Prod-1, Prod-2) в шифрованный локальный файл config и быстро переключаться между ними.
 [ ] Индикатор «Production Safeguard»:
  - [ ] Визуальное предупреждение (например, красная рамка или подтверждение через ввод имени кластера) при выполнении опасных действий (Failover / Reinit) на Prodbase.
 
### Лицензия

Распространяется под лицензией **MIT License**.
