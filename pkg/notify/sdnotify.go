// Package notify wraps systemd sd_notify protocol for the Vorn daemon.
// On non-Linux systems or when not running under systemd, calls are no-ops.
package notify

import (
	"fmt"
	"net"
	"os"
)

// Ready tells systemd the daemon is fully initialized and ready for traffic.
// Corresponds to sd_notify("READY=1").
func Ready() error {
	return sdNotify("READY=1")
}

// Watchdog sends a keepalive to systemd's watchdog timer.
// Must be called at intervals less than WatchdogSec/2.
func Watchdog() error {
	return sdNotify("WATCHDOG=1")
}

// Status sends a human-readable status string to systemd.
// Visible via `systemctl status vornd`.
func Status(msg string) error {
	return sdNotify(fmt.Sprintf("STATUS=%s", msg))
}

// Reloading tells systemd the daemon is reloading its configuration.
// Call Ready() again when reload is complete.
func Reloading() error {
	return sdNotify("RELOADING=1")
}

// Stopping tells systemd the daemon is beginning its shutdown sequence.
func Stopping() error {
	return sdNotify("STOPPING=1")
}

// sdNotify sends a notification to the systemd notification socket.
// Returns nil if NOTIFY_SOCKET is not set (not running under systemd).
func sdNotify(state string) error {
	socketAddr := os.Getenv("NOTIFY_SOCKET")
	if socketAddr == "" {
		return nil // not running under systemd
	}

	conn, err := net.Dial("unixgram", socketAddr)
	if err != nil {
		return fmt.Errorf("sd_notify dial: %w", err)
	}
	defer conn.Close()

	if _, err := conn.Write([]byte(state)); err != nil {
		return fmt.Errorf("sd_notify write: %w", err)
	}
	return nil
}
