#!/bin/bash
# Pre-job hook: GPU reset, VRAM clear, workspace preparation.
# Called by vornd before launching a job container.
# Environment variables: JOB_ID, GPU_INDICES, WORKSPACE_DIR
set -euo pipefail

JOB_ID="${JOB_ID:-unknown}"
GPU_INDICES="${GPU_INDICES:-}"
WORKSPACE_DIR="${WORKSPACE_DIR:-}"

echo "[pre-job] Starting for job=${JOB_ID} gpus=${GPU_INDICES}"

# Reset GPU compute state (clear any residual processes)
if [ -n "${GPU_INDICES}" ]; then
    IFS=',' read -ra GPUS <<< "${GPU_INDICES}"
    for gpu in "${GPUS[@]}"; do
        # Reset GPU application clocks to default
        nvidia-smi -i "${gpu}" --reset-gpu-clocks 2>/dev/null || true
        nvidia-smi -i "${gpu}" --reset-applications-clocks 2>/dev/null || true

        # Clear any GPU compute processes from previous jobs
        nvidia-smi -i "${gpu}" --compute-mode=DEFAULT 2>/dev/null || true

        echo "[pre-job] GPU ${gpu} reset complete"
    done
fi

# Prepare workspace directories
if [ -n "${WORKSPACE_DIR}" ]; then
    mkdir -p "${WORKSPACE_DIR}/input"
    mkdir -p "${WORKSPACE_DIR}/output"
    mkdir -p "${WORKSPACE_DIR}/tmp"
    chmod 755 "${WORKSPACE_DIR}"
    echo "[pre-job] Workspace prepared at ${WORKSPACE_DIR}"
fi

echo "[pre-job] Complete for job=${JOB_ID}"
