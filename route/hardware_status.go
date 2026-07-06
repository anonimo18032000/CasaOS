package route

import (
	"fmt"
	"sync"
	"time"

	"github.com/IceWhaleTech/CasaOS/pkg/config"
	"github.com/IceWhaleTech/CasaOS/service"
)

var (
	hardwareStatusMu     sync.Mutex
	hardwareStatusTicker *time.Ticker
	hardwareStatusStop   chan struct{}
)

// InitHardwareStatusCron starts the periodic push of CPU/memory/network stats to the dashboard,
// using whatever interval is currently stored in config.ServerInfo.HardwareStatusIntervalMs.
func InitHardwareStatusCron() {
	// route/v1 handlers can't import this package directly (it would be an import cycle, since
	// this package already imports route/v1) - they go through this indirection in service instead.
	service.RescheduleHardwareStatusFunc = RescheduleHardwareStatus

	_ = RescheduleHardwareStatus(config.ServerInfo.HardwareStatusIntervalMs)
}

// RescheduleHardwareStatus changes how often CPU/memory/network stats are pushed to the
// dashboard, replacing whatever schedule was previously active.
//
// This intentionally does not use robfig/cron's `@every` scheduling: cron's ConstantDelaySchedule
// silently floors any duration below one second up to exactly 1s (see its Every() constructor),
// so a sub-second interval like "250ms" would quietly run at 1s instead. A plain time.Ticker has
// no such floor.
func RescheduleHardwareStatus(intervalMs int) error {
	if intervalMs <= 0 {
		return fmt.Errorf("interval must be positive")
	}

	hardwareStatusMu.Lock()
	defer hardwareStatusMu.Unlock()

	if hardwareStatusTicker != nil {
		hardwareStatusTicker.Stop()
		close(hardwareStatusStop)
	}

	ticker := time.NewTicker(time.Duration(intervalMs) * time.Millisecond)
	stop := make(chan struct{})
	hardwareStatusTicker = ticker
	hardwareStatusStop = stop

	go func() {
		for {
			select {
			case <-ticker.C:
				SendAllHardwareStatusBySocket()
			case <-stop:
				return
			}
		}
	}()

	return nil
}
