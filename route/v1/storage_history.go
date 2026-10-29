package v1

import (
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/IceWhaleTech/CasaOS/model"
	"github.com/IceWhaleTech/CasaOS/pkg/utils/common_err"
	"github.com/labstack/echo/v4"
)

const (
	healthSampleEvery = time.Minute
	healthKeep        = 24 * time.Hour
	// a manual refresh of the health panel also records a sample, but never closer than this to the
	// previous one, so opening the panel repeatedly doesn't distort the graph
	healthMinSpacing = 10 * time.Second
)

// HealthSample is one reachability/latency measurement of a storage.
type HealthSample struct {
	Time      int64 `json:"t"` // unix milliseconds
	Healthy   bool  `json:"ok"`
	LatencyMs int64 `json:"ms"`
}

type healthHistory struct {
	mu      sync.Mutex
	samples map[string][]HealthSample // keyed by mount point
}

var storageHealthHistory = &healthHistory{samples: map[string][]HealthSample{}}

func (h *healthHistory) record(mountPoint string, sample HealthSample) {
	h.mu.Lock()
	defer h.mu.Unlock()

	list := h.samples[mountPoint]
	if n := len(list); n > 0 && sample.Time-list[n-1].Time < healthMinSpacing.Milliseconds() {
		return
	}
	list = append(list, sample)

	cutoff := sample.Time - healthKeep.Milliseconds()
	drop := 0
	for drop < len(list) && list[drop].Time < cutoff {
		drop++
	}
	h.samples[mountPoint] = list[drop:]
}

// forget drops history of mount points that no longer exist.
func (h *healthHistory) forget(alive map[string]bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for mountPoint := range h.samples {
		if !alive[mountPoint] {
			delete(h.samples, mountPoint)
		}
	}
}

func (h *healthHistory) since(mountPoint string, from int64) map[string][]HealthSample {
	h.mu.Lock()
	defer h.mu.Unlock()

	result := map[string][]HealthSample{}
	for mp, list := range h.samples {
		if mountPoint != "" && mp != mountPoint {
			continue
		}
		i := sort.Search(len(list), func(i int) bool { return list[i].Time >= from })
		result[mp] = append([]HealthSample{}, list[i:]...)
	}
	return result
}

func recordStorageHealth(results []StorageHealth) {
	now := time.Now().UnixMilli()
	alive := map[string]bool{}
	for _, r := range results {
		alive[r.MountPoint] = true
		storageHealthHistory.record(r.MountPoint, HealthSample{Time: now, Healthy: r.Healthy, LatencyMs: r.LatencyMs})
	}
	storageHealthHistory.forget(alive)
}

// StartStorageHealthSampler measures every mounted storage once a minute so the health panel can
// draw latency over time. History lives in memory only: it is a diagnostic aid, not a record.
func StartStorageHealthSampler() {
	go func() {
		for {
			time.Sleep(healthSampleEvery)
			if results, err := checkStorages(); err == nil {
				recordStorageHealth(results)
			}
		}
	}()
}

// GetStoragesHealthHistory returns the recorded samples, optionally for one mount point and limited
// to the last `minutes` (default 60, at most 24h).
func GetStoragesHealthHistory(ctx echo.Context) error {
	minutes, err := strconv.Atoi(ctx.QueryParam("minutes"))
	if err != nil || minutes <= 0 {
		minutes = 60
	}
	if limit := int(healthKeep / time.Minute); minutes > limit {
		minutes = limit
	}
	from := time.Now().Add(-time.Duration(minutes) * time.Minute).UnixMilli()
	data := storageHealthHistory.since(ctx.QueryParam("mount_point"), from)
	return ctx.JSON(common_err.SUCCESS, model.Result{Success: common_err.SUCCESS, Message: common_err.GetMsg(common_err.SUCCESS), Data: data})
}
