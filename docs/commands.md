# RolloutGuardian Command Reference

This document provides a comprehensive guide to all terminal commands, build instructions, CLI subcommands, and operational workflows used in the **RolloutGuardian** project.

---

## Table of Contents
1. [Prerequisites & Environment Setup](#1-prerequisites--environment-setup)
2. [Build & Testing Commands](#2-build--testing-commands)
3. [CLI Subcommands (`rolloutguardian`)](#3-cli-subcommands-rolloutguardian)
   - [evaluate](#31-evaluate--governance-gate-evaluation)
   - [explain](#32-explain--signal-breakdown--reasoning)
   - [scorecard](#33-scorecard--resilience-readiness-grading)
   - [backtest](#34-backtest--historical-impact-simulation)
4. [Long-Running Gate Service & Web Dashboard](#4-long-running-gate-service--web-dashboard)
5. [Agentic MCP Adapter (`mcp-adapter`)](#5-agentic-mcp-adapter-mcp-adapter)
6. [Docker & Containerized Deployment](#6-docker--containerized-deployment)

---

## 1. Prerequisites & Environment Setup

Before running CLI commands against live Harness APIs, configure your local environment or `.rolloutguardian.yaml` configuration file.

### Initial Configuration
```bash
# Clone the repository and initialize the configuration file
cp .rolloutguardian.yaml.example .rolloutguardian.yaml
```

### Environment Variables (Bash / Zsh)
```bash
export HARNESS_API_KEY="pat.xxxxx"
export HARNESS_ACCOUNT_ID="xxxxx"
```

### Environment Variables (PowerShell / Windows)
```powershell
$env:HARNESS_API_KEY="pat.xxxxx"
$env:HARNESS_ACCOUNT_ID="xxxxx"
```

> **Note:** If `HARNESS_API_KEY` is not set, RolloutGuardian automatically falls back to **Mock Client mode**, loading local fixture data from `examples/catalog-fixtures/catalog.json` and `examples/catalog-fixtures/chaos-map.json`. This allows full local testing without requiring credentials.

---

## 2. Build & Testing Commands

RolloutGuardian is written in Go 1.23+ and uses standard Go toolchain commands.

### Build All Packages & Binaries
Verify that all core modules, CLI binaries, and server implementations build without error:
```bash
go build ./...
```

### Run the Full Unit Test Suite
Execute all unit tests across all 10 internal packages with verbose output:
```bash
go test ./... -v -count=1
```

### Run Tests for Specific Packages
```bash
# Test the OPA/Rego policy evaluation engine
go test ./internal/policy -v

# Test the blast radius resolution algorithms
go test ./internal/resolver -v

# Test the historical backtest engine
go test ./backtest -v
```

### Test Rego Policies Directly (`opa` CLI)
If you have the Open Policy Agent (`opa`) CLI installed, you can execute Rego tests directly:
```bash
opa test policies/rolloutguardian -v
```

---

## 3. CLI Subcommands (`rolloutguardian`)

You can run the CLI directly from source using `go run ./cmd/rolloutguardian` or compile a binary:
```bash
# Optional: compile binary to local directory
go build -o rolloutguardian.exe ./cmd/rolloutguardian
```

### 3.1 `evaluate` — Governance Gate Evaluation
Evaluates whether a proposed feature-flag rollout change is safe to proceed based on downstream Chaos resilience scores and SRM error budgets.

```bash
# Standard interactive/dry-run evaluation
go run ./cmd/rolloutguardian evaluate \
  --flag checkout-v2-express-pay \
  --target-rollout 50 \
  --dry-run
```

#### JSON Output (for CI/CD pipelines & automation)
```bash
go run ./cmd/rolloutguardian evaluate \
  --flag checkout-v2-express-pay \
  --target-rollout 50 \
  --json \
  --dry-run
```

#### Flags:
- `--flag <key>`: The target Feature Management & Experimentation (FME) flag key (Required).
- `--target-rollout <pct>`: The proposed rollout percentage `0-100` (Required).
- `--config <path>`: Path to configuration YAML (Default: `.rolloutguardian.yaml`).
- `--dry-run`: Evaluate policy without applying changes or triggering side effects.
- `--json`: Output full `EvaluationPayload` and decision reasoning in structured JSON format.

---

### 3.2 `explain` — Signal Breakdown & Reasoning
Prints a detailed human-readable audit trail explaining *why* a flag's downstream blast radius resulted in its current decision (`ALLOW`, `WARN`, or `BLOCK`).

```bash
go run ./cmd/rolloutguardian explain --flag checkout-v2-express-pay
```

#### Example Output Snippet:
```text
============================================================
RolloutGuardian Explain Report: checkout-v2-express-pay
============================================================
Flag Key:        checkout-v2-express-pay
Owning Service:  checkout-service
Resolved Blast Radius:
  - Service:          payment-service
    Confidence:       0.94 (trace_association)
    Chaos Coverage:   118 days old (Resilience Score: nil)
    Error Budget:     12.4% remaining (SLO: payment-service-availability)
```

---

### 3.3 `scorecard` — Resilience Readiness Grading
Generates an organization-wide scorecard grading every service across the application map on a scale from **A to F** based on coverage freshness and remaining error budget.

```bash
go run ./cmd/rolloutguardian scorecard
```

#### Grading Criteria:
- **Grade A:** Fresh Chaos coverage (`<= 90d`) AND Error Budget `>= 25%`.
- **Grade B:** Fresh Chaos coverage AND Error Budget between `10%` and `25%`.
- **Grade C:** Stale Chaos coverage (`> 90d`) OR Error Budget between `10%` and `25%`.
- **Grade F:** Stale Chaos coverage AND Error Budget `< 10%`.

---

### 3.4 `backtest` — Historical Impact Simulation
Replays historical rollout-change timestamps against incident timelines to measure how many previous Sev1/Sev2 incidents RolloutGuardian would have successfully flagged before impact.

```bash
# Human-readable summary table
go run ./cmd/rolloutguardian backtest

# Structured JSON metrics output
go run ./cmd/rolloutguardian backtest --json
```

---

## 4. Long-Running Gate Service & Web Dashboard

For real-time integration into Harness CI/CD pipelines (via `HTTP / evaluate` steps) or browser-based visualization, run the API server and UI dashboard.

### Start the Server
```bash
go run ./cmd/rolloutguardian-server --config .rolloutguardian.yaml --port 8080
```

### Accessing the UI Dashboard
Open your browser and navigate to:
```text
http://localhost:8080/
```
The dashboard provides live visual simulation of rollout percentage adjustments (`0-100%`) and displays real-time `ALLOW` / `WARN` / `BLOCK` decision updates with suggested remediation links.

### API Endpoints (`curl` / Pipeline Steps)
```bash
# 1. Pipeline Evaluation Hook (POST /evaluate)
curl -X POST http://localhost:8080/evaluate \
  -H "Content-Type: application/json" \
  -d '{"flag_key": "checkout-v2-express-pay", "from_pct": 25, "to_pct": 50}'

# Equivalent payload using the rollout-percentage alias
curl -X POST http://localhost:8080/evaluate \
  -H "Content-Type: application/json" \
  -d '{"flag_key": "checkout-v2-express-pay", "target_rollout_pct": 50}'

# 2. Interactive Simulation Hook (POST /api/simulate)
curl -X POST http://localhost:8080/api/simulate \
  -H "Content-Type: application/json" \
  -d '{"flag_key": "checkout-v2-express-pay", "from_pct": 25, "to_pct": 75}'

# 3. Health Check (GET /healthz)
curl http://localhost:8080/healthz
```

---

## 5. Agentic MCP Adapter (`mcp-adapter`)

The TypeScript Model Context Protocol (MCP) server exposes `rolloutguardian_evaluate` and `rolloutguardian_explain` tools to AI agents (such as Claude Desktop or Harness AI Assistant).

### Install Node.js Dependencies
```bash
cd mcp-adapter
npm install
```

### Build TypeScript Server
```bash
npm run build
```

### Run the MCP Server Directly
```bash
npm start
# Equivalent: node dist/index.js
```

### MCP Server Tool Definitions
- **`rolloutguardian_evaluate`**: Takes `{ flag_key: string, target_rollout_pct: number }` and returns the full resilience-aware decision object (`allow`/`warn`/`block`).
- **`rolloutguardian_explain`**: Takes `{ flag_key: string }` and returns the human-readable audit reasoning for the downstream blast radius.

---

## 6. Docker & Containerized Deployment

To run RolloutGuardian as a containerized service inside Kubernetes or local Docker:

### Build and Run with Docker Compose
```bash
docker-compose up --build
```
This starts the `rolloutguardian-server` container on port `8080`. Policies and catalog fixtures are mounted into `/root` (the image working directory) so fixture-backed evaluations pick up local edits.
