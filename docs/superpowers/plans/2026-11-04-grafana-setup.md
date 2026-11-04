# Grafana Setup Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Run a persistent, local-only Grafana service that a developer can manually connect to the existing InfluxDB 3 Core database with SQL.

**Architecture:** Add Grafana as an ordinary Compose service on the existing default network. Grafana reaches InfluxDB through the `influxdb3` service name; a host bind mount retains Grafana's embedded SQLite state. The project deliberately leaves datasource and dashboard provisioning out of this first integration.

**Tech Stack:** Docker Compose, Grafana OSS image `grafana/grafana:13.2.2`, Grafana's built-in InfluxDB SQL data source, InfluxDB 3 Core 3.11.

**Spec:** `docs/superpowers/specs/2026-11-04-grafana-setup-design.md`

## Global Constraints

- Before execution, create and work exclusively in an isolated Git worktree; do not modify the primary checkout.
- Execute with `superpowers:subagent-driven-development`: use a fresh implementer for the task, a fresh reviewer before accepting it, and a final whole-branch review.
- Use the pinned official image `grafana/grafana:13.2.2`, not `latest` or the no-longer-updated `grafana/grafana-oss` repository.
- Expose Grafana only as `127.0.0.1:3000:3000`; retain InfluxDB's existing loopback-only host port.
- Add `./volumes/grafana:/var/lib/grafana` and retain its contents across ordinary Compose rebuild/recreation. Run Grafana as root solely so the bind mount is writable on local Docker hosts.
- Do not add datasource/dashboard provisioning, a dashboard, a new Grafana plugin, backend changes, or credentials to committed configuration.
- The manual datasource must use `http://influxdb3:8181`, `InfluxDB Enterprise 3.x`, SQL, `busyness_sensing`, and insecure gRPC. Use a read-only token once InfluxDB authentication is enabled.

## Review Focus

- The Grafana port must not bind to all host interfaces; the Compose-config inspection owns this check.
- Container recreation must retain Grafana state through `./volumes/grafana`; the live acceptance check owns this.
- Grafana must connect via the Compose service hostname, never the host loopback address; the README walkthrough owns this.
- Local TLS is absent, so the data source must enable the SQL insecure connection setting; the manual Save & test check owns this.
- No datasource, dashboard, or token may be provisioned or committed; the diff review owns this.

---

### Task 1: Add and verify the local Grafana service

**Files:**
- Modify: `docker-compose.yml`
- Modify: `.gitignore`
- Create: `volumes/grafana/.gitkeep`
- Modify: `README.md`
- Create during planning: `docs/superpowers/specs/2026-11-04-grafana-setup-design.md`
- Create during planning: `docs/superpowers/plans/2026-11-04-grafana-setup.md`

**Interfaces:**
- Consumes: the existing `influxdb3` Compose service and its `service_healthy` health check.
- Produces: `grafana` Compose service at `http://127.0.0.1:3000` with its durable state at `volumes/grafana`.

- [ ] **Step 1:** Inspect the existing Compose configuration with `docker compose config`; record its successful baseline or any environment blocker before editing.
- [ ] **Step 2:** Add `grafana` using `grafana/grafana:13.2.2`, `restart: unless-stopped`, `depends_on.influxdb3.condition: service_healthy`, root user, loopback-only port `127.0.0.1:3000:3000`, and bind mount `./volumes/grafana:/var/lib/grafana`. Add the ignored-but-tracked empty runtime directory following the existing Mosquitto/InfluxDB `.gitkeep` pattern.
- [ ] **Step 3:** Extend the README local-services section with the Grafana URL, initial-login flow, and exact manual InfluxDB SQL settings. State that the bind mount preserves data sources and dashboards across normal rebuild/recreation, and that deleting `volumes/grafana` is the explicit reset. Do not document a dashboard.
- [ ] **Step 4:** Run `docker compose config` and assert the rendered Grafana service has the pinned image, healthy InfluxDB dependency, exact loopback port, and exact bind mount. Run `git diff --check`; expect both checks to pass.
- [ ] **Step 5:** If Docker is available, run `docker compose up -d --build`, wait for `grafana` to be running, and verify `http://127.0.0.1:3000` returns the Grafana sign-in page. In the UI, create the manual InfluxDB SQL data source using the specified settings and confirm **Save & test** succeeds. Recreate only the Grafana container with `docker compose up -d --force-recreate grafana`, sign back in, and confirm the data source remains. If the daemon/UI is unavailable, record the exact blocker without claiming live verification.
- [ ] **Step 6:** Commit the implementation and planning documents with `git add docker-compose.yml .gitignore volumes/grafana/.gitkeep README.md docs/superpowers/specs/2026-11-04-grafana-setup-design.md docs/superpowers/plans/2026-11-04-grafana-setup.md` and message `feat: add persistent local grafana`.
