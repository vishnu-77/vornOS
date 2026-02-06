#!/bin/bash
# Post-job hook: cleanup, artifact collection, GPU state verification.
# Called by vornd after a job container exits.
# Environment variables: JOB_ID, GPU_INDICES, WORKSPACE_DIR, JOB_STATUS, JOB_EXIT_CODE
set -euo pipefail

JOB_ID="${JOB_ID:-unknown}"
GPU_INDICES="${GPU_INDICES:-}"
WORKSPACE_DIR="${WORKSPACE_DIR:-}"
JOB_STATUS="${JOB_STATUS:-unknown}"
JOB_EXIT_CODE="${JOB_EXIT_CODE:-}"

echo "[post-job] Starting for job=${JOB_ID} status=${JOB_STATUS} exit=${JOB_EXIT_CODE}"

# Verify GPU state after job
if [ -n "${GPU_INDICES}" ]; then
    IFS=',' read -ra GPUS <<< "${GPU_INDICES}"
    for gpu in "${GPUS[@]}"; do
        # Check for leftover compute processes
        procs=$(nvidia-smi -i "${gpu}" --query-compute-apps=pid --format=csv,noheader 2>/dev/null || echo "")
        if [ -n "${procs}" ]; then
            echo "[post-job] WARNING: GPU ${gpu} still has compute processes: ${procs}"
        fi

        # Log post-job GPU memory state
        mem_used=$(nvidia-smi -i "${gpu}" --query-gpu=memory.used --format=csv,noheader,nounits 2>/dev/null || echo "N/A")
        echo "[post-job] GPU ${gpu} memory used after job: ${mem_used} MiB"
    done
fi

# Clean up workspace tmp directory
if [ -n "${WORKSPACE_DIR}" ] && [ -d "${WORKSPACE_DIR}/tmp" ]; then
    rm -rf "${WORKSPACE_DIR}/tmp"
    echo "[post-job] Cleaned workspace tmp"
fi

# Collect output artifacts size
if [ -n "${WORKSPACE_DIR}" ] && [ -d "${WORKSPACE_DIR}/output" ]; then
    artifact_size=$(du -sh "${WORKSPACE_DIR}/output" 2>/dev/null | cut -f1 || echo "0")
    echo "[post-job] Output artifacts size: ${artifact_size}"
fi

echo "[post-job] Complete for job=${JOB_ID}"
