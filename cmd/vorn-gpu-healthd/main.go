// vorn-gpu-healthd is a standalone GPU health monitor daemon.
// It polls nvidia-smi for health metrics, detects faults (ECC errors,
// thermal throttling, Xid errors), and logs events.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/vornos/buildos/pkg/config"
	"github.com/vornos/buildos/pkg/gpu"
	"github.com/vornos/buildos/pkg/logging"
)

func main() {
	cfgPath := flag.String("config", "/etc/vorn/vornd.yaml", "path to config file")
	flag.Parse()

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "config: %v\n", err)
		os.Exit(1)
	}

	logger, err := logging.NewLogger(cfg.LogDir, cfg.LogLevel)
	if err != nil {
		fmt.Fprintf(os.Stderr, "logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()
	log := logger.Sugar()

	log.Infow("starting vorn-gpu-healthd", "poll_interval_sec", cfg.GPUHealthPollSec)

	// Discover GPUs
	inventory, err := gpu.Discover()
	if err != nil {
		log.Fatalf("gpu discovery: %v", err)
	}
	log.Infow("monitoring GPUs", "count", len(inventory.Devices))

	// Open health log
	healthLogPath := cfg.LogDir + "/gpu-health.log"
	healthLog, err := os.OpenFile(healthLogPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0640)
	if err != nil {
		log.Fatalf("open health log: %v", err)
	}
	defer healthLog.Close()

	// Health event callback
	onEvent := func(event gpu.HealthEvent) {
		log.Warnw("gpu health event",
			"device", event.DeviceIndex,
			"old_state", event.OldState,
			"new_state", event.NewState,
			"reason", event.Reason,
		)

		// Write to dedicated health log as JSON
		entry := map[string]interface{}{
			"timestamp":    event.Timestamp.Format(time.RFC3339Nano),
			"device_index": event.DeviceIndex,
			"old_state":    event.OldState,
			"new_state":    event.NewState,
			"reason":       event.Reason,
		}
		data, _ := json.Marshal(entry)
		data = append(data, '\n')
		healthLog.Write(data)
	}

	// Start health monitor
	monitor := gpu.NewHealthMonitor(
		inventory,
		time.Duration(cfg.GPUHealthPollSec)*time.Second,
		onEvent,
	)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := monitor.Start(ctx); err != nil && err != context.Canceled {
		log.Fatalf("health monitor: %v", err)
	}
}
