# Vorn OS gRPC API Reference

## Connection

### TCP (with optional mTLS)
```
Address: 0.0.0.0:9800 (configurable via api_addr)
TLS: Optional, configured in /etc/vorn/tls/
```

### Unix Socket (local only)
```
Path: /var/run/vorn/vornd.sock
Permissions: 0660, owner vorn:vorn
```

`vornctl` prefers the Unix socket, falling back to TCP if unavailable.

## Service Definition

```protobuf
service VornService {
    // System
    rpc GetStatus(Empty) returns (StatusResponse);
    rpc GetHealth(Empty) returns (HealthResponse);
    rpc GetGPUInventory(Empty) returns (GPUInventoryResponse);

    // Runtimes
    rpc RegisterRuntime(RegisterRuntimeRequest) returns (RuntimeResponse);
    rpc ListRuntimes(Empty) returns (ListRuntimesResponse);
    rpc InspectRuntime(InspectRuntimeRequest) returns (RuntimeResponse);
    rpc RemoveRuntime(RemoveRuntimeRequest) returns (Empty);

    // Models
    rpc AddModel(stream AddModelRequest) returns (ModelResponse);
    rpc ListModels(Empty) returns (ListModelsResponse);
    rpc VerifyModel(VerifyModelRequest) returns (VerifyModelResponse);
    rpc RemoveModel(RemoveModelRequest) returns (Empty);

    // Jobs
    rpc SubmitJob(SubmitJobRequest) returns (JobResponse);
    rpc ListJobs(ListJobsRequest) returns (ListJobsResponse);
    rpc GetJob(GetJobRequest) returns (JobResponse);
    rpc StreamJobLogs(StreamJobLogsRequest) returns (stream LogLine);
    rpc KillJob(KillJobRequest) returns (Empty);
}
```

## RPCs

### GetStatus
Returns daemon version, uptime, GPU count, active jobs, and subsystem health.

**Request**: `Empty`
**Response**:
```json
{
  "version": "0.1.0",
  "uptime_seconds": 3600,
  "gpu_count": 4,
  "active_jobs": 2,
  "status": "ready"
}
```

### GetHealth
Liveness check. Returns subsystem statuses.

### GetGPUInventory
Returns full GPU inventory with per-device details.

**Response**:
```json
{
  "devices": [
    {
      "index": 0,
      "uuid": "GPU-abc123",
      "name": "NVIDIA A100-SXM4-80GB",
      "memory_total_mb": 81920,
      "memory_used_mb": 1024,
      "utilization_pct": 45,
      "temperature_c": 62,
      "power_watts": 250,
      "driver_version": "550.54.14",
      "compute_cap": "sm_80"
    }
  ]
}
```

### RegisterRuntime
Register a new runtime layer (Apptainer SIF or OCI image).

**Request**:
```json
{
  "name": "pytorch-2.2",
  "image_path": "/path/to/pytorch-2.2.sif",
  "framework": "pytorch",
  "framework_version": "2.2.0",
  "cuda_version": "12.4"
}
```

### AddModel (streaming)
Upload and register a signed model. Streamed in chunks for large files.

**Request stream**:
1. First message: metadata (name, format, framework, signature, signer key ID)
2. Subsequent messages: binary chunks (up to 4MB each)

**Response**: Model ID, SHA-256 hash, verification status

### SubmitJob
Submit a new job for execution.

**Request**:
```json
{
  "model_id": "llama-7b",
  "runtime_id": "pytorch-2.2",
  "gpu_count": 2,
  "vram_mb": 16384,
  "mode": "research",
  "command": ["python3", "inference.py"],
  "env": {"BATCH_SIZE": "32"},
  "timeout_seconds": 3600
}
```

**Response**: Job ID, status, assigned GPUs

### StreamJobLogs
Server-streaming RPC that tails job stdout/stderr in real-time.

**Request**: `{ "job_id": "job-abc123", "follow": true }`
**Response stream**: `{ "timestamp": "...", "stream": "stdout", "line": "..." }`

### KillJob
Send SIGTERM to a running job. If the job doesn't exit within 10 seconds, SIGKILL is sent.

**Request**: `{ "job_id": "job-abc123" }`

## Error Codes

| gRPC Code | Meaning |
|-----------|---------|
| `NOT_FOUND` | Runtime, model, or job ID doesn't exist |
| `ALREADY_EXISTS` | Duplicate runtime/model name |
| `INVALID_ARGUMENT` | Bad request parameters |
| `FAILED_PRECONDITION` | Model verification failed, GPU unavailable |
| `RESOURCE_EXHAUSTED` | Insufficient VRAM or GPU slots |
| `PERMISSION_DENIED` | mTLS auth failure |
| `INTERNAL` | Unexpected server error |

## Middleware

### Logging Interceptor
All RPC calls are logged with method, duration, and status code.

### Recovery Interceptor
Panics in handlers are caught and returned as `INTERNAL` errors.

### Audit
Security-relevant operations (model add/verify, job submit/kill) are recorded in the audit log.

## Health Endpoints (HTTP)

Available on the metrics port (default 9801):

| Endpoint | Purpose | Success |
|----------|---------|---------|
| `GET /healthz` | Liveness probe | `{"status":"ok"}` |
| `GET /readyz` | Readiness probe | `{"status":"ready","checks":{...}}` |
| `GET /metrics` | Prometheus scrape | Prometheus text format |
