# InfluxDB Health Split Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `IsHealthy` accept the InfluxDB SDK client and make `Init` call it.

**Architecture:** Keep configuration validation and SDK client ownership in `Init`. Configure the SDK's HTTP client with a five-second timeout and redirect rejection, then use its `GetServerVersion` method in `IsHealthy` to check `/ping`.

**Tech Stack:** Go 1.27, InfluxDB 3 Go SDK v2.17.0, `net/http`.

**Spec:** `docs/superpowers/specs/2026-10-26-influxdb-health-split-design.md`

## Global Constraints

- Preserve the five-second timeout, Bearer token, redirect rejection, sanitized errors, and client close on health failure. InfluxDB Core's successful `GET /ping` replaces the raw `GET /health` check.
- Keep the `Init() (*influxdb3.Client, error)` signature and its use in `cmd/start.go`.
- Use the existing isolated `codex/influxdb-connection` worktree; do not touch the primary checkout.
- Run Go tests with `GOCACHE=/private/tmp/codex-influx-go-build GOPROXY=off` and local HTTP listener approval where required.

## Review Focus

- `Init` must pass the constructed SDK client to `IsHealthy`, check its error with an `if` statement, and close the SDK client before returning that error.
- `IsHealthy` should be callable directly with a client and report success, non-200, redirect, nil-client, and transport failures without revealing the token.
- No change to startup configuration, Compose behavior, or the returned client type.

---

### Task 1: Make IsHealthy client-based

**Files:**
- Modify: `internal/driven/database/influxdb/init.go`
- Modify: `internal/driven/database/influxdb/init_test.go`
- Modify: `README.md`
- Modify: `AGENTS.md`

**Interfaces:**
- Produces: `func IsHealthy(client *influxdb3.Client) error`.
- `Init()` calls `if err := IsHealthy(client); err != nil` after constructing the SDK client.

- [ ] Change a direct `IsHealthy` test to pass a real SDK client and model `GET /ping` success/failure; confirm the test fails to compile before the signature change.
- [ ] Run `go test ./internal/driven/database/influxdb` and confirm the expected red failure.
- [ ] Configure the SDK client HTTP timeout/redirect policy in `Init`. Implement `IsHealthy(client *influxdb3.Client) error` using `GetServerVersion`, sanitizing errors and preserving HTTP status codes where available; have `Init` call it in an `if err := ...` statement and close its client on error.
- [ ] Update direct and `Init` tests for `/ping`, including redirect, nil-client, and transport failure. Keep real `httptest` fixtures and test that the token is not exposed.
- [ ] Update README and AGENTS.md startup descriptions to name `/ping` for the backend and `/health` for Compose.
- [ ] Run focused tests, `go test ./...`, `go vet ./...`, and `git diff --check`; commit.
