# RolloutGuardian — Native Harness Custom Step

RolloutGuardian can be embedded directly into any Harness CD or Feature Flags (FME) pipeline as a native **Harness Custom Step**. This enables automated, zero-touch resilience gating: before any flag rollout percentage is increased or targeting rule enabled, RolloutGuardian evaluates real-time Chaos Engineering coverage and Service Reliability Management (SRM) error budget burn rates across the high-confidence blast radius.

## Files

- `step-template.yaml` — The Harness Step Template specification (`CustomStep` type v1.0.0).
- `Dockerfile` — Builds the lightweight `rolloutguardian/step-runner:v1.0.0` container image.
- `entrypoint.sh` — Executes the gate evaluation, outputs pipeline step variables (`decision`, `blast_radius_size`, `remediation_manifest`), and enforces pipeline halts if `enforce_mode: true`.

## Quickstart & Import into Harness

1. In your Harness Account or Project, navigate to **Templates** -> **New Template** -> **Step Template**.
2. Paste the contents of `step-template.yaml` or point to this Git repository branch.
3. Add the step to your deployment pipeline prior to the **FME Flag Rollout Step**:
   ```yaml
   - step:
       type: CustomStep
       name: Verify Resilience Gate
       identifier: verify_resilience_gate
       template:
         templateRef: rolloutguardian_resilience_gate
         versionLabel: v1.0.0
         templateInputs:
           flag_key: "checkout-v2-express-pay"
           to_pct: 50
           enforce_mode: true
   ```

## Output Variables

When the step completes, the following variables are exported to `<+pipeline.stages.[stage].spec.execution.steps.verify_resilience_gate.output.outputVariables[name]>`:

| Variable | Description | Example |
|---|---|---|
| `decision` | Governance decision computed by OPA Rego (`allow`, `warn`, or `block`) | `block` |
| `blast_radius_size` | Count of downstream services identified in the blast radius | `2` |
| `remediation_manifest` | Path to generated Kubernetes experiment manifest if blocked | `examples/experiments/generated/payment-service-pod-delete-min.yaml` |
