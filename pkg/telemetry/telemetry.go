// Package telemetry exposes Prometheus metrics for GPU usage,
// job latency, model-level counters, scheduler, API, and daemon health.
package telemetry

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/vornos/buildos/pkg/gpu"
)

// Collector holds all VRM metrics.
type Collector struct {
	inventory *gpu.Inventory
	mu        sync.RWMutex

	// GPU metrics
	gpuUtilization *prometheus.GaugeVec
	gpuMemoryUsed  *prometheus.GaugeVec
	gpuMemoryTotal *prometheus.GaugeVec
	gpuTemperature *prometheus.GaugeVec
	gpuPowerWatts  *prometheus.GaugeVec
	gpuECCErrors   *prometheus.GaugeVec
	gpuHealthState *prometheus.GaugeVec
	gpuCount       prometheus.Gauge

	// Job metrics
	jobsTotal       *prometheus.CounterVec
	jobDuration     *prometheus.HistogramVec
	jobsActive      prometheus.Gauge
	jobQueueDepth   prometheus.Gauge
	jobStartLatency prometheus.Histogram

	// Model metrics
	modelRequests  *prometheus.CounterVec
	modelFailures  *prometheus.CounterVec
	modelLatency   *prometheus.HistogramVec
	modelsStored   prometheus.Gauge
	modelSizeBytes *prometheus.GaugeVec

	// Scheduler metrics
	schedulerVRAMAllocated *prometheus.GaugeVec
	schedulerVRAMTotal     *prometheus.GaugeVec
	schedulerGPUsAllocated prometheus.Gauge
	schedulerGPUsTotal     prometheus.Gauge

	// API metrics
	apiRequestsTotal    *prometheus.CounterVec
	apiRequestDuration  *prometheus.HistogramVec
	apiActiveStreams     prometheus.Gauge
	apiConnectionsTotal prometheus.Counter

	// Daemon metrics
	daemonUptime       prometheus.Counter
	daemonStartTime    prometheus.Gauge
	daemonConfigReload *prometheus.CounterVec
	auditEventsTotal   *prometheus.CounterVec

	// Runtime metrics
	runtimesStored prometheus.Gauge
	runtimePulls   *prometheus.CounterVec
}

// NewCollector creates and registers all VRM metrics.
func NewCollector(inv *gpu.Inventory) *Collector {
	c := &Collector{
		inventory: inv,

		// ── GPU ──
		gpuUtilization: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: "vorn", Subsystem: "gpu", Name: "utilization_percent",
			Help: "GPU compute utilization percentage",
		}, []string{"gpu_index", "gpu_uuid"}),

		gpuMemoryUsed: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: "vorn", Subsystem: "gpu", Name: "memory_used_mb",
			Help: "GPU memory used in MB",
		}, []string{"gpu_index", "gpu_uuid"}),

		gpuMemoryTotal: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: "vorn", Subsystem: "gpu", Name: "memory_total_mb",
			Help: "GPU total memory in MB",
		}, []string{"gpu_index", "gpu_uuid"}),

		gpuTemperature: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: "vorn", Subsystem: "gpu", Name: "temperature_celsius",
			Help: "GPU temperature in Celsius",
		}, []string{"gpu_index", "gpu_uuid"}),

		gpuPowerWatts: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: "vorn", Subsystem: "gpu", Name: "power_watts",
			Help: "GPU power draw in watts",
		}, []string{"gpu_index", "gpu_uuid"}),

		gpuECCErrors: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: "vorn", Subsystem: "gpu", Name: "ecc_errors_total",
			Help: "GPU ECC error count",
		}, []string{"gpu_index", "gpu_uuid", "type"}),

		gpuHealthState: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: "vorn", Subsystem: "gpu", Name: "health_state",
			Help: "GPU health state (0=healthy, 1=degraded, 2=failed)",
		}, []string{"gpu_index", "gpu_uuid", "state"}),

		gpuCount: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "vorn", Subsystem: "gpu", Name: "count",
			Help: "Total number of GPUs detected",
		}),

		// ── Jobs ──
		jobsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "vorn", Subsystem: "jobs", Name: "total",
			Help: "Total number of jobs by status",
		}, []string{"status"}),

		jobDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: "vorn", Subsystem: "jobs", Name: "duration_seconds",
			Help:    "Job duration in seconds",
			Buckets: prometheus.ExponentialBuckets(0.1, 2, 15),
		}, []string{"model_id", "runtime_id"}),

		jobsActive: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "vorn", Subsystem: "jobs", Name: "active",
			Help: "Number of currently running jobs",
		}),

		jobQueueDepth: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "vorn", Subsystem: "jobs", Name: "queue_depth",
			Help: "Number of jobs waiting to be scheduled",
		}),

		jobStartLatency: prometheus.NewHistogram(prometheus.HistogramOpts{
			Namespace: "vorn", Subsystem: "jobs", Name: "start_latency_seconds",
			Help:    "Time from job submission to execution start",
			Buckets: prometheus.ExponentialBuckets(0.01, 2, 12),
		}),

		// ── Models ──
		modelRequests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "vorn", Subsystem: "model", Name: "requests_total",
			Help: "Total model load/inference requests",
		}, []string{"model_id"}),

		modelFailures: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "vorn", Subsystem: "model", Name: "failures_total",
			Help: "Total model verification or load failures",
		}, []string{"model_id"}),

		modelLatency: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: "vorn", Subsystem: "model", Name: "inference_latency_seconds",
			Help:    "Model inference latency in seconds",
			Buckets: prometheus.ExponentialBuckets(0.001, 2, 20),
		}, []string{"model_id"}),

		modelsStored: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "vorn", Subsystem: "model", Name: "stored_count",
			Help: "Number of models in the store",
		}),

		modelSizeBytes: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: "vorn", Subsystem: "model", Name: "size_bytes",
			Help: "Size of stored model in bytes",
		}, []string{"model_id"}),

		// ── Scheduler ──
		schedulerVRAMAllocated: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: "vorn", Subsystem: "scheduler", Name: "vram_allocated_mb",
			Help: "VRAM allocated on each GPU in MB",
		}, []string{"gpu_index"}),

		schedulerVRAMTotal: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: "vorn", Subsystem: "scheduler", Name: "vram_total_mb",
			Help: "Total VRAM on each GPU in MB",
		}, []string{"gpu_index"}),

		schedulerGPUsAllocated: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "vorn", Subsystem: "scheduler", Name: "gpus_allocated",
			Help: "Number of GPUs with active allocations",
		}),

		schedulerGPUsTotal: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "vorn", Subsystem: "scheduler", Name: "gpus_total",
			Help: "Total GPUs available to scheduler",
		}),

		// ── API ──
		apiRequestsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "vorn", Subsystem: "api", Name: "requests_total",
			Help: "Total API requests by method and code",
		}, []string{"method", "code"}),

		apiRequestDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: "vorn", Subsystem: "api", Name: "request_duration_seconds",
			Help:    "API request latency in seconds",
			Buckets: prometheus.DefBuckets,
		}, []string{"method"}),

		apiActiveStreams: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "vorn", Subsystem: "api", Name: "active_streams",
			Help: "Number of active streaming RPCs",
		}),

		apiConnectionsTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "vorn", Subsystem: "api", Name: "connections_total",
			Help: "Total gRPC connections established",
		}),

		// ── Daemon ──
		daemonUptime: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "vorn", Subsystem: "daemon", Name: "uptime_seconds_total",
			Help: "Daemon uptime in seconds (incremented by scrape goroutine)",
		}),

		daemonStartTime: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "vorn", Subsystem: "daemon", Name: "start_time_seconds",
			Help: "Daemon start time as Unix timestamp",
		}),

		daemonConfigReload: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "vorn", Subsystem: "daemon", Name: "config_reloads_total",
			Help: "Config reload attempts by result",
		}, []string{"result"}),

		auditEventsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "vorn", Subsystem: "audit", Name: "events_total",
			Help: "Audit events by level",
		}, []string{"level", "event_type"}),

		// ── Runtimes ──
		runtimesStored: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "vorn", Subsystem: "runtime", Name: "stored_count",
			Help: "Number of runtime layers in the store",
		}),

		runtimePulls: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "vorn", Subsystem: "runtime", Name: "pulls_total",
			Help: "Runtime layer pull/register operations",
		}, []string{"runtime_id", "result"}),
	}

	// Register all metrics
	prometheus.MustRegister(
		// GPU
		c.gpuUtilization, c.gpuMemoryUsed, c.gpuMemoryTotal, c.gpuTemperature,
		c.gpuPowerWatts, c.gpuECCErrors, c.gpuHealthState, c.gpuCount,
		// Jobs
		c.jobsTotal, c.jobDuration, c.jobsActive, c.jobQueueDepth, c.jobStartLatency,
		// Models
		c.modelRequests, c.modelFailures, c.modelLatency, c.modelsStored, c.modelSizeBytes,
		// Scheduler
		c.schedulerVRAMAllocated, c.schedulerVRAMTotal, c.schedulerGPUsAllocated, c.schedulerGPUsTotal,
		// API
		c.apiRequestsTotal, c.apiRequestDuration, c.apiActiveStreams, c.apiConnectionsTotal,
		// Daemon
		c.daemonUptime, c.daemonStartTime, c.daemonConfigReload, c.auditEventsTotal,
		// Runtimes
		c.runtimesStored, c.runtimePulls,
	)

	return c
}

// RegisterHandlers adds the /metrics endpoint to the given mux.
func (c *Collector) RegisterHandlers(mux *http.ServeMux) {
	mux.Handle("/metrics", promhttp.Handler())
}

// StartGPUScraper launches a goroutine that periodically refreshes GPU metrics.
func (c *Collector) StartGPUScraper(ctx context.Context, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				c.scrapeGPU()
			}
		}
	}()
}

// scrapeGPU reads the latest GPU inventory and updates all GPU gauges.
func (c *Collector) scrapeGPU() {
	c.mu.RLock()
	inv := c.inventory
	c.mu.RUnlock()

	if inv == nil {
		return
	}

	c.gpuCount.Set(float64(len(inv.Devices)))

	for _, dev := range inv.Devices {
		idx := fmt.Sprintf("%d", dev.Index)
		uuid := dev.UUID

		c.gpuUtilization.WithLabelValues(idx, uuid).Set(float64(dev.UtilizationPct))
		c.gpuMemoryUsed.WithLabelValues(idx, uuid).Set(float64(dev.MemoryUsedMB))
		c.gpuMemoryTotal.WithLabelValues(idx, uuid).Set(float64(dev.MemoryTotalMB))
		c.gpuTemperature.WithLabelValues(idx, uuid).Set(float64(dev.TemperatureC))
		c.gpuPowerWatts.WithLabelValues(idx, uuid).Set(float64(dev.PowerWatts))
	}
}

// SetDaemonStartTime records the daemon start time.
func (c *Collector) SetDaemonStartTime(t time.Time) {
	c.daemonStartTime.Set(float64(t.Unix()))
}

// ── Recording helpers ──

// RecordJobStart should be called when a job begins execution.
func (c *Collector) RecordJobStart() {
	c.jobsActive.Inc()
}

// RecordJobEnd should be called when a job finishes.
func (c *Collector) RecordJobEnd(status, modelID, runtimeID string, durationSec float64) {
	c.jobsActive.Dec()
	c.jobsTotal.WithLabelValues(status).Inc()
	c.jobDuration.WithLabelValues(modelID, runtimeID).Observe(durationSec)
}

// RecordJobQueued increments the queue depth.
func (c *Collector) RecordJobQueued() {
	c.jobQueueDepth.Inc()
}

// RecordJobDequeued decrements the queue depth and records scheduling latency.
func (c *Collector) RecordJobDequeued(waitSec float64) {
	c.jobQueueDepth.Dec()
	c.jobStartLatency.Observe(waitSec)
}

// RecordModelRequest increments the request counter for a model.
func (c *Collector) RecordModelRequest(modelID string) {
	c.modelRequests.WithLabelValues(modelID).Inc()
}

// RecordModelFailure increments the failure counter for a model.
func (c *Collector) RecordModelFailure(modelID string) {
	c.modelFailures.WithLabelValues(modelID).Inc()
}

// SetModelsStored sets the total models in the store.
func (c *Collector) SetModelsStored(n int) {
	c.modelsStored.Set(float64(n))
}

// SetRuntimesStored sets the total runtime layers.
func (c *Collector) SetRuntimesStored(n int) {
	c.runtimesStored.Set(float64(n))
}

// RecordAPIRequest records an API call with method and status code.
func (c *Collector) RecordAPIRequest(method, code string, durationSec float64) {
	c.apiRequestsTotal.WithLabelValues(method, code).Inc()
	c.apiRequestDuration.WithLabelValues(method).Observe(durationSec)
}

// RecordAPIStreamStart increments active stream count.
func (c *Collector) RecordAPIStreamStart() {
	c.apiActiveStreams.Inc()
}

// RecordAPIStreamEnd decrements active stream count.
func (c *Collector) RecordAPIStreamEnd() {
	c.apiActiveStreams.Dec()
}

// RecordAPIConnection increments connection count.
func (c *Collector) RecordAPIConnection() {
	c.apiConnectionsTotal.Inc()
}

// RecordConfigReload tracks config reload attempts.
func (c *Collector) RecordConfigReload(result string) {
	c.daemonConfigReload.WithLabelValues(result).Inc()
}

// RecordAuditEvent tracks audit log events.
func (c *Collector) RecordAuditEvent(level, eventType string) {
	c.auditEventsTotal.WithLabelValues(level, eventType).Inc()
}

// SetSchedulerVRAM sets allocated/total VRAM for a GPU in the scheduler.
func (c *Collector) SetSchedulerVRAM(gpuIndex int, allocatedMB, totalMB uint64) {
	idx := fmt.Sprintf("%d", gpuIndex)
	c.schedulerVRAMAllocated.WithLabelValues(idx).Set(float64(allocatedMB))
	c.schedulerVRAMTotal.WithLabelValues(idx).Set(float64(totalMB))
}

// SetSchedulerGPUs sets the allocated/total GPU counts.
func (c *Collector) SetSchedulerGPUs(allocated, total int) {
	c.schedulerGPUsAllocated.Set(float64(allocated))
	c.schedulerGPUsTotal.Set(float64(total))
}

// SetGPUHealth sets the health state gauge for a specific GPU.
func (c *Collector) SetGPUHealth(gpuIndex int, uuid string, state string, value float64) {
	idx := fmt.Sprintf("%d", gpuIndex)
	c.gpuHealthState.WithLabelValues(idx, uuid, state).Set(value)
}

// UpdateInventory atomically swaps the inventory reference for scraping.
func (c *Collector) UpdateInventory(inv *gpu.Inventory) {
	c.mu.Lock()
	c.inventory = inv
	c.mu.Unlock()
}
