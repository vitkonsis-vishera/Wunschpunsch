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
git clone https://github.com/your-username/wunschpunsch.git
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

**Main Dashboard (Tabs 1–6)**
* `1` .. `6` — Jump directly to tab (`1: Topology`, `2: Metrics`, `3: Analytics`, `4: Actions`, `5: Event Logs`, `6: Settings`)
* `Tab` / `l` / `Right` — Next tab
* `Shift+Tab` / `h` / `Left` — Previous tab
* `e` / `E` — Export metrics snapshot (JSON / CSV)
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
- [x] **Global Settings Menu (Modal / Settings Tab):**
  - [x] On-the-fly UI Language Switching (English / Russian) via i18n.
  - [x] Color Scheme Selection (Catppuccin, Nord, Monokai).
  - [x] Configurable Cluster Polling Interval (Default: 2s).
- [x] **Data Export & Logging:**
  - [x] Local Metrics Snapshot Export (JSON / CSV).
  - [x] Event Log Dump & Auto-saving to local file (`~/.wunschpunsch/logs/app.log`).
- [x] **Sub-tab Filtering inside the Logs Viewport:**
  - [x] `[All]` — Unified real-time event stream.
  - [x] `[Postgres]` — Database-specific metrics, warnings, and errors.
  - [x] `[HA Engine / Patroni]` — Role switches, timeline changes, and failover health checks.
  - [x] `[System / Host]` — System alerts (high CPU/RAM/Disk utilization).
  - [x] [SSH / Journal] — Dedicated stream for systemd service logs (journalctl) & SSH log collection.
- [x] **Interactive Log Features:**
  - [x] In-stream Keyword Search & Filtering (via `/` hotkey with match highlighting).
  - [x] Pause Auto-scroll on scroll-up (`a` to resume).
- [x] **Long Query & Lock Monitoring (Active Queries & Diagnostics):**
  - [x] View slow SQL queries directly from `pg_stat_statements`.
  - [x] Highlight hung transactions (`idle in transaction` / >2s) and active locks from `pg_stat_activity`.
  - [x] Safely cancel/terminate long-running backends (`pg_cancel_backend` via `k` / `pg_terminate_backend` via `K`) directly from the TUI.
- [x] **Maintenance & DBA Operations Toolkit:**
  - [x] Interactive execution of core DBA operations (`CHECKPOINT`, `ANALYZE`, `VACUUM ANALYZE`, killing idle sessions, query stat resets, `VACUUM FULL`).
  - [x] Risk classification (`LOW`, `MEDIUM`, `HIGH`) and interactive safety confirmation modal dialog.
- [x] **Patroni Dynamic Configuration (DCS Viewer):**
  - [x] Inspect dynamic cluster configuration (DCS `/config`) in modal window (`c` key).
- [x] **Node-level Control & Restart:**
  - [x] Interactive node menu for managing and restarting cluster members (`Enter` key on table row).
- [x] **Automated Health & Resource Alerting:**
  - [x] Automatic background detection for node unreachability, high replication lag (>100 MB), and host CPU spikes (>85%) with auto-logging.
- [x] **Remote SSH & Local Journal Log Integration:**
  - [x] Full interactive SSH configuration fields (User, Port, SSH Key Path, Password) with active cursor navigation.
  - [x] Automatic fallback to native local log reader (journalctl / sh) for localhost and 127.0.0.1 endpoints without SSH connection overhead.
  - [x] Password authentication via sshpass alongside standard SSH public keys.
- [x] **Bloat & Autovacuum Tracker:**
  - [x] Monitor active `autovacuum` workers and table/index bloat ratios.
- [x] **Corosync + Pacemaker Full Integration:**
  - [x] Auto-detection and resource status visualization for `crm_mon` / `pcs status` (initial client and actions integrated).
- [x] **PgBouncer / HAProxy Monitoring:**
  - [x] Dedicated widget for connection pool status (active client/server pools, waiting clients via `SHOW POOLS`).
- [x] **Connection Manager:**
  - [x] Save and manage multiple cluster environments (Staging, Prod-1, Prod-2) in configuration file for quick switching.
- [x] **Production Safeguard Mode:**
  - [x] Visual high-visibility alerts (prominent red borders and mandatory cluster name confirmation) when performing destructive/critical actions on Production instances.
- [x] **Kerberos / GSSAPI & Keytab Support:**
  - [x] Interactive GSSAPI/Kerberos connection checkbox with automatic `kinit` execution via `keytab` file and `pg_hba.conf` integration for closed MS AD / FreeIPA enterprise environments.
- [x] **Query Planning & Analytics (EXPLAIN):**
  - [x] Interactive query execution plans generation (`EXPLAIN`) for analytical diagnostics.
- [x] **Advanced DBA & Cluster Operations:**
  - [x] **Advanced Replication Lag & LSN Visualizer:**
    - [x] Real-time tracking of WAL senders and receivers (`pg_stat_replication`), LSN divergence charts, and recovery time estimation.
  - [x] **Interactive Query Profiling & EXPLAIN ANALYZE:**
    - [x] Deep execution analysis using buffers, execution time breakdown, and bottleneck visualization directly in the TUI.
  - [x] **Automated Index Recommendations & Missing Index Detector:**
    - [x] Sequential scan scanning (`pg_stat_user_tables`) and hints for creating missing indexes with `CREATE INDEX` SQL auto-generation.
  - [x] **Deadlock Detector & Transaction Tree Visualization:**
    - [x] Real-time monitoring of blocked queries and deadlock graphs with root-cause session highlighting.
  - [x] **Patroni Switchover / Failover Scheduler & Pre-checks:**
    - [x] Pre-switchover validation (replication lag, sync status, disk space) prior to leader change.
  - [x] **Prometheus & Grafana Metrics Exporter Integration:**
    - [x] Built-in lightweight metrics endpoint (`/metrics`) for external scraping or proxying cluster telemetry.
  - [x] **Encrypted Credentials Storage (Vault / AES-256):**
    - [x] Secure local password storage with master password encryption or OS keychain integration.
  - [x] **Custom Webhooks for Alerts & Slack / Telegram Notifications:**
    - [x] Threshold-based alerting configuration for replication lag, CPU spikes, and node failures directly to webhook endpoints.
  - [x] **Schema Migrations & DDL History Audit Log:**
    - [x] Schema change tracking (`CREATE`, `ALTER`, `DROP`) via event triggers and system audit logs.
  - [x] **Standby Cluster Health Matrix:**
    - [x] Cross-region replica health monitoring with inter-datacenter latency metrics.
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

**Основные вкладки (1–6)**
* `1` .. `6` — Быстрый переход по вкладкам (`1: Топология`, `2: Метрики`, `3: Аналитика`, `4: Действия`, `5: Логи`, `6: Настройки`)
* `Tab` / `l` / `Right` — Следующая вкладка
* `Shift+Tab` / `h` / `Left` — Предыдущая вкладка
* `e` / `E` — Экспорт снимка метрик (JSON / CSV)
* `Ctrl+C` / `q` — Выход из приложения

### План разработки (Roadmap)

- [x] Авторизация и подключение к Patroni + PostgreSQL
- [x] Фоновое обновление состояния кластера каждые 2 секунды
- [x] Таблица топологии узлов (Lag, Timeline, Roles)
- [x] Сбор метрик PostgreSQL (Cache Hit Ratio, соединения)
- [x] **Управление кластером из TUI:**
  - [x] Плановый (Switchover) и принудительный (Failover) смен лидера
  - [x] Реинициализация реплики (Reinitialize)
  - [x] Переключение режима обслуживания (Pause/Resume)
- [x] Просмотр потока логов событий Patroni в реальном времени
- [x] **Улучшения UI & Настройки (App Settings):**
  - [x] Экран глобальных настроек (Settings Modal/Tab).
  - [x] Переключение языка интерфейса (RU / EN) «на лету» (i18n).
  - [x] Выбор цветовой схемы (Catppuccin, Nord, Monokai).
  - [x] Настройка интервала опрашивания кластера (default: 2s).
- [x] **Экспорт данных & Логирование:**
  - [x] Сохранение текущего снимка метрик в JSON / CSV на локальную машину.
  - [x] Дамп и автосохранение истории событий логов в файл (`~/.wunschpunsch/logs/app.log`).
- [x] **Вкладки фильтрации (Sub-tabs) внутри Viewport логов:**
  - [x] `[All]` — единый поток всех событий.
  - [x] `[Postgres]` — логи и метрики базы данных.
  - [x] `[HA Engine / Patroni]` — события смены ролей, таймлайнов, healthchecks.
  - [x] `[System / Host]` — системные события и предупреждения (CPU/RAM/Disk).
  - [x] [SSH / Journal] — отдельная подвкладка системных логов служб (journalctl) и SSH-подключений.
- [x] **Интеграция SSH и локального чтения системных логов:**
  - [x] Интерактивные текстовые поля SSH (User, Port, Key Path, Password) с отображением фокуса и курсора.
  - [x] Автоматическое переключение на локальные журналы (journalctl) при подключении к localhost / 127.0.0.1 без необходимости поднятия SSH-сессии.
  - [x] Поддержка парольного подключения через sshpass и аутентификации по ключам SSH.
- [x] **Интерактивные возможности логов:**
  - [x] Текстовый поиск и фильтрация по ключевым словам по клавише `/` с подсветкой совпадений.
  - [x] Пауза автоскролла логов при прокрутке вверх (`a` для возобновления).
- [x] **Мониторинг долгих запросов и блокировок (Active Queries / Вкладка Analytics):**
  - [x] Просмотр топ медленных SQL-запросов из `pg_stat_statements`.
  - [x] Подсветка зависших транзакций (`idle in transaction` / >2с) и активных блокировок из `pg_stat_activity`.
  - [x] Возможность завершить долгий запрос прямо из TUI: мягкая отмена `pg_cancel_backend` (`k`) / принудительное завершение `pg_terminate_backend` (`K`).
- [x] **Инструменты DBA и операции обслуживания (DBA Operations):**
  - [x] Выполнение операций обслуживания (`CHECKPOINT`, `ANALYZE`, `VACUUM ANALYZE`, отстрел зависших idle-сессий, сброс статистики, `VACUUM FULL`) с фиксацией времени выполнения.
  - [x] Градация по уровням риска (`LOW`, `MEDIUM`, `HIGH`) и интерактивный диалог подтверждения перед запуском.
- [x] **Просмотр динамической конфигурации Patroni (DCS Viewer):**
  - [x] Просмотр актуальной DCS-конфигурации (`/config`) в модальном окне по клавише `c`.
- [x] **Управление узлами кластера (Node-level Control):**
  - [x] Меню управления выбранным узлом и его перезапуск через Patroni API (клавиша `Enter` на строке таблицы).
- [x] **Автоматическая система алертов и мониторинга ресурсов:**
  - [x] Автодетект недоступности узлов, отставания репликации > 100 MB и всплесков CPU хоста > 85% с фиксацией в журнале логов.
- [x] **Анализ Bloat и очистки (Autovacuum Tracker):**
  - [x] Отображение текущих процессов autovacuum и уровня раздувания таблиц/индексов.
- [x] **Полная поддержка Corosync + Pacemaker:**
  - [x] Детект и отображение ресурсов `crm_mon` / `pcs status` для Pacemaker-кластеров (клиент и команды интегрированы).
- [x] **Мониторинг PgBouncer / HAProxy:**
  - [x] Отдельный виджет состояния пулера соединений (количество активных клиентских и серверных пулов, waiting clients через `SHOW POOLS`).
- [x] **Менеджер профилей (Connection Manager):**
  - [x] Возможность сохранять несколько конфигураций кластеров (Staging, Prod-1, Prod-2) в локальный файл config и быстро переключаться между ними.
- [x] **Индикатор «Production Safeguard»:**
  - [x] Визуальное предупреждение (например, красная рамка или подтверждение через ввод имени кластера) при выполнении опасных действий (Failover / Reinit) на Production-базах.
- [x] **Поддержка GSSAPI / Kerberos & Keytab:**
  - [x] Чекбокс Kerberos на экране подключения с поддержкой автоматического выполнения `kinit` по указанному файлу `keytab` и настройкой доступа в `pg_hba.conf` для закрытых корпоративных контуров (MS AD / FreeIPA).
- [x] **Анализ планов запросов (EXPLAIN):**
  - [x] Интерактивное построение планов выполнения запросов (`EXPLAIN`) для глубокой аналитики.

### Продвинутый DBA и операции с кластером
- [x] **Визуализатор отставания репликации и LSN (Advanced Replication Lag & LSN Visualizer):**
  - [x] Отслеживание в реальном времени WAL senders и receivers (`pg_stat_replication`), расхождение LSN и байтовый лаг.
- [x] **Интерактивное профилирование запросов и EXPLAIN ANALYZE:**
  - [x] Глубокий анализ выполнения с использованием буферов, детализацией времени выполнения и визуализацией узких мест прямо в TUI.
- [x] **Автоматические рекомендации индексов и детектор пропущенных индексов:**
  - [x] Сканирование sequential scans (`pg_stat_user_tables`) и подсказки по созданию недостающих индексов с автогенерацией `CREATE INDEX` SQL-запросов.
- [x] **Детектор взаимоблокировок (Deadlocks) и древовидная визуализация транзакций:**
  - [x] Мониторинг заблокированных запросов в реальном времени и графы дедлоков с подсветкой сессии-первопричины.
- [x] **Планировщик переключений Patroni (Switchover / Failover Scheduler) и предварительные проверки:**
  - [x] Валидация перед переключением (отставание репликации, статус синхронизации, свободное место на диске) до смены лидера.
- [x] **Интеграция с Prometheus & Grafana Metrics Exporter:**
  - [x] Встроенный легковесный эндпоинт метрик (`/metrics`) для внешнего сбора или проксирования телеметрии кластера.
- [x] **Зашифрованное хранилище учетных данных (Vault / AES-256):**
  - [x] Безопасное хранение локальных паролей с шифрованием по мастер-паролю или интеграцией с системными связками ключей ОС.
- [x] **Кастомные вебхуки для алертов и уведомления в Slack / Telegram:**
  - [x] Настройка пороговых уведомлений об отставании репликации, всплесках CPU и падении узлов напрямую на вебхук-эндпоинты.
- [x] **Миграции схемы и аудит-лог истории DDL:**
  - [x] Отслеживание изменений схемы (`CREATE`, `ALTER`, `DROP`) через event triggers и системные журналы аудита.
- [x] **Матрица здоровья распределенного резервного кластера (Standby Cluster Health Matrix):**
  - [x] Межрегиональный мониторинг состояния реплик с метриками задержки между дата-центрами.

### Лицензия

Распространяется под лицензией **MIT License**.
