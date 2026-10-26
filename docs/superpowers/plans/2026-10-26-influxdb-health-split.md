# InfluxDB Health Split Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Extract the HTTP health check into an exported `IsHealthy` function and make `Init` call it.

**Architecture:** Keep configuration validation and SDK client ownership in `Init`. Pass host and token explicitly to `IsHealthy` for the existing HTTP health request.

**Tech Stack:** Go 1.27, InfluxDB 3 Go SDK v2.17.0, `net/http`.

**Spec:** `docs/superpowers/specs/2026-10-26-influxdb-health-split-design.md`

## Global Constraints

- Preserve the existing five-second timeout, Bearer token, redirect rejection, sanitized errors, HTTP 200 requirement, and client close on health failure.
- Keep the `Init() (*influxdb3.Client, error)` signature and its use in `cmd/start.go`.
- Use the existing isolated `codex/influxdb-connection` worktree; do not touch the primary checkout.
- Run Go tests with `GOCACHE=/private/tmp/codex-influx-go-build GOPROXY=off` and local HTTP listener approval where required.

## Review Focus

- `Init` must check `IsHealthy` error with an `if` statement and close the SDK client before returning that error.
- `IsHealthy` should be callable directly and report success, non-200, redirect, and transport failures without revealing the token.
- No change to startup configuration, Compose behavior, or the returned client type.

---

### Task 1: Extract IsHealthy

**Files:**
- Modify: `internal/driven/database/influxdb/init.go`
- Modify: `internal/driven/database/influxdb/init_test.go`

**Interfaces:**
- Produces: `func IsHealthy(host, token string) error`.
- `Init()` calls `if err := IsHealthy(c.Host, c.Token); err != nil` after constructing the SDK client.

- [ ] Add a direct `IsHealthy` test using `httptest` that fails to compile before extraction and covers HTTP 200 success and HTTP 503 failure.
- [ ] Run `go test ./internal/driven/database/influxdb` and confirm the expected red failure.
- [ ] Move the existing HTTP health request logic into `IsHealthy(host, token string) error`; have `Init` call it in an `if err := ...` statement and close its client on error.
- [ ] Extend direct health tests for redirect and transport failure only where they prove the extracted function's behavior; keep existing `Init` tests for ownership and cleanup behavior.
- [ ] Run focused tests, `go test ./...`, `go vet ./...`, and `git diff --check`; commit.
