# RolloutGuardian Architecture

RolloutGuardian is a resilience-aware release governance engine designed to sit between Harness Feature Management & Experimentation (FME) rollout decisions and actual user traffic. It connects FME, Harness Chaos Engineering, and Harness Service Reliability Management (SRM) into a unified Open Policy Agent (OPA/Rego) decision pipeline.

## Core Components

### 1. Blast Radius Resolver
Determines which downstream services are impacted when a feature flag evaluation result changes. Uses three complementary detection layers:
- **Static Call-Site Analysis**: Scans application source code for SDK evaluations matching the target flag key.
- **Trace-Based Statistical Association**: Mines OpenTelemetry trace spans and performs non-parametric Mann-Whitney U tests (`is_in_blast_radius`) to test if downstream service latencies differ significantly when the flag is enabled versus disabled.
- **Structural Dependency Fallback**: Walks the IDP software catalog / Chaos application map up to `max_hops` away from the owning service to establish structural coverage when traffic samples are low.

Each resolved service node is assigned a `confidence` score (0.0 to 1.0) and a `detection_method` label.

### 2. Signal Aggregator
For every service within the resolved blast radius, the Aggregator collects runtime resilience metrics:
- **Chaos Engineering Signal**: Fetches the most recent resilience score (`resilience_score`) and calculates the freshness in days (`days_since_last_result`).
- **SRM Signal**: Fetches active Service Level Objectives (SLOs) along with remaining error budget percentage (`error_budget_remaining_pct`) and burn rate.
- **STO Signal**: Optionally aggregates count of critical and high security findings.

### 3. OPA Decision Engine
Evaluates aggregated service signals against Rego governance policies (`authz.rego`). Outputs an auditable decision:
- `block`: When Chaos coverage is stale (`> 90 days`) **and** error budget is critically depleted (`< 10%`), or when open critical STO findings are present.
- `warn`: When error budget is below healthy threshold (`< 25%`) but above marginal (`>= 10%`), or when Chaos coverage is stale but the error budget is still healthy.
- `allow`: When all blast radius services satisfy coverage and budget policies.

### 4. Remediation Generator
When a `block` decision occurs, RolloutGuardian generates a minimal, precisely targeted Harness Chaos Experiment manifest (`.yaml`) to run against the uncovered service (e.g., pod-delete or network-latency), streamlining the unblock path for product and SRE teams.

### 5. Backtest Engine
Replays historical rollout timestamp events against past incident logs to quantify how many historical Sev1/Sev2 incidents would have been caught before production impact.
