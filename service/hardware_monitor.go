package service

import "fmt"

// Bounds for the dashboard refresh rate: faster than this only burns CPU and floods the socket,
// slower makes the dashboard look frozen.
const (
	MinHardwareStatusIntervalMs     = 250
	MaxHardwareStatusIntervalMs     = 60000
	DefaultHardwareStatusIntervalMs = 5000
)

// RescheduleHardwareStatusFunc is wired up by the route package at startup, which owns the actual
// cron job that pushes CPU/memory/network stats to the dashboard. route/v1 handlers can't import
// route directly (route already imports route/v1, so that would be an import cycle), so they go
// through this indirection instead.
var RescheduleHardwareStatusFunc func(intervalMs int) error

// RescheduleHardwareStatus changes how often hardware stats are pushed to the dashboard.
func RescheduleHardwareStatus(intervalMs int) error {
	if RescheduleHardwareStatusFunc == nil {
		return fmt.Errorf("hardware status scheduler is not ready yet")
	}
	return RescheduleHardwareStatusFunc(intervalMs)
}
