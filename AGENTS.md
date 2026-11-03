# Agent Guidelines for Wunschpunsch

This document provides essential information, commands, architecture patterns, and conventions for working effectively in the **Wunschpunsch** codebase.

---

## 🚀 Essential Commands

- **Build Native Binary:**
  ```bash
  CGO_ENABLED=0 go build -ldflags="-s -w" -o wunschpunsch .
  ```
- **Run Tests:**
  ```bash
  go test -v ./...
  ```
- **Run Static Analysis (Vet):**
  ```bash
  go vet ./...
  ```
- **Run Locally:**
  ```bash
  go run main.go
  ```

---

## 🏛️ Code Organization & Architecture

Wunschpunsch is a terminal-based management console (TUI) written in Go using the Charm ecosystem (`bubbletea`, `lipgloss`, `bubbles`) for monitoring and managing high-availability **PostgreSQL** clusters orchestrated by **Patroni**, **Pacemaker/Corosync**, and **PgBouncer**.

### Directory Structure
- Root package (`main.go`, `model.go`, `commands.go`, `views.go`, `actions.go`, `types.go`, `utils.go`, `metrics.go`): Contains the main Bubble Tea model loop, state management, UI rendering views, command handlers, and UI actions.
- `pkg/postgres/`: PostgreSQL database connection pools, pool manager, metrics collection, analytics, slow query diagnostics, replication lag, and missing index detectors.
- `pkg/cluster/`: Cluster manager abstractions and Patroni/Pacemaker cluster state providers.
- `pkg/patroni/`: Patroni REST API client and DCS (Distributed Consensus Store) configuration client.
- `pkg/pacemaker/`: Pacemaker/Corosync `crm_mon`/`pcs status` client and parser.
- `pkg/ssh/`: Remote SSH client wrapper and log streaming facilities.
- `pkg/config/`: Application configuration file loader and environment settings manager.
- `pkg/exporter/`: Prometheus metrics scraper and HTTP exporter endpoint.
- `pkg/i18n/`: Internationalization layer supporting English and Russian translations.

---

## 💡 Code Conventions & Style Patterns

- **Language & Version:** Go `1.25.0`.
- **Concurrency & State:** Bubble Tea Elm-like architecture with a central `Model` struct. Asynchronous operations are driven via `tea.Cmd` returning custom message types processed in the `Update` loop.
- **Error Handling:** Standard Go error returns with explicit handling and logging. Errors from background tasks or queries are funneled into the UI status / event log streams.
- **Naming Conventions:** Standard Go naming conventions (camelCase for local/unexported fields, PascalCase for exported types and methods).

---

## 🧪 Testing Approach

- Standard Go unit tests (`*_test.go`) residing alongside package code.
- Run tests across the whole project using `go test ./...`.

---

## ⚠️ Important Gotchas & Non-Obvious Patterns

- **TUI Async Loops:** UI refresh cycles and Patroni polling intervals run via `tea.Tick` commands. Avoid blocking main goroutines inside the Bubble Tea `Update` function; always wrap long-running network or database calls in `tea.Cmd` functions.
- **Cross-Platform Compilation:** Binaries for release builds are typically compiled with `CGO_ENABLED=0` to ensure static linking.
- **Credential & Config Management:** Supports both plaintext local settings and encrypted credentials (AES-256 vault / keychain integration). Ensure sensitive values are not logged in output views or debug statements.
