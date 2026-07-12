#!/bin/sh
set -e

echo "================================================================================"
echo "RolloutGuardian Harness Custom Step Runner v1.0.0"
echo "================================================================================"
echo "Evaluating Flag Key: ${FLAG_KEY}"
echo "Requested Operation: ${REQUESTED_CHANGE:-increase_rollout} (${FROM_PCT:-0}% -> ${TO_PCT:-50}%)"
echo "Enforce Mode: ${ENFORCE_MODE:-true}"
echo "--------------------------------------------------------------------------------"

# Run rolloutguardian CLI evaluation in JSON mode
OUTPUT_JSON=$(rolloutguardian evaluate \
  --flag-key="${FLAG_KEY}" \
  --from-rollout="${FROM_PCT:-0}" \
  --target-rollout="${TO_PCT:-50}" \
  --json)

DECISION=$(echo "${OUTPUT_JSON}" | grep -o '"decision": *"[^"]*"' | head -1 | cut -d'"' -f4 || echo "allow")
BLAST_SIZE=$(echo "${OUTPUT_JSON}" | grep -o '"service": *"[^"]*"' | wc -l || echo "0")
REMEDIAL_PATH=$(echo "${OUTPUT_JSON}" | grep -o '"suggested_remediation": *"[^"]*"' | head -1 | cut -d'"' -f4 || echo "")

echo "Governance Evaluation Decision: $(echo ${DECISION} | tr '[:lower:]' '[:upper:]')"
echo "Resolved Blast Radius Size: ${BLAST_SIZE} service(s)"
if [ -n "${REMEDIAL_PATH}" ]; then
  echo "Suggested Remediation Manifest: ${REMEDIAL_PATH}"
fi

# Export to Harness step output variables
if [ -n "${HARNESS_OUTPUT_PATH}" ]; then
  echo "decision=${DECISION}" >> "${HARNESS_OUTPUT_PATH}"
  echo "blast_radius_size=${BLAST_SIZE}" >> "${HARNESS_OUTPUT_PATH}"
  echo "remediation_manifest=${REMEDIAL_PATH}" >> "${HARNESS_OUTPUT_PATH}"
fi

echo "================================================================================"

if [ "${DECISION}" = "block" ] && [ "${ENFORCE_MODE}" = "true" ]; then
  echo "[!] PIPELINE HALTED BY ROLLOUTGUARDIAN: Governance policy evaluation returned BLOCK."
  echo "[!] Check downstream Chaos Engineering coverage and SRM error budget levels."
  if [ -n "${REMEDIAL_PATH}" ]; then
    echo "[!] Run remedial experiment or check: ${REMEDIAL_PATH}"
  fi
  exit 1
fi

echo "[+] Rollout check passed (${DECISION}). Proceeding with deployment."
exit 0
