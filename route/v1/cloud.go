package v1

import (
	"os"
	"strings"
	"sync"
	"time"

	"github.com/IceWhaleTech/CasaOS-Common/utils/logger"
	"github.com/IceWhaleTech/CasaOS/drivers/dropbox"
	"github.com/IceWhaleTech/CasaOS/drivers/google_drive"
	"github.com/IceWhaleTech/CasaOS/drivers/onedrive"
	"github.com/IceWhaleTech/CasaOS/model"
	"github.com/IceWhaleTech/CasaOS/pkg/utils/common_err"
	"github.com/IceWhaleTech/CasaOS/pkg/utils/httper"
	"github.com/IceWhaleTech/CasaOS/service"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

type StorageHealth struct {
	ID         string `json:"id"`
	MountPoint string `json:"mount_point"`
	Name       string `json:"name"`
	Type       string `json:"type"`
	Healthy    bool   `json:"healthy"`
	LatencyMs  int64  `json:"latency_ms"`
	Error      string `json:"error,omitempty"`
	Host       string `json:"host,omitempty"`
	Port       string `json:"port,omitempty"`
	Username   string `json:"username,omitempty"`
	RemotePath string `json:"remote_path,omitempty"`
}

func ListStorages(ctx echo.Context) error {
	r, err := service.MyService.Storage().GetStorages()
	if err != nil {
		return ctx.JSON(common_err.SUCCESS, model.Result{Success: common_err.SERVICE_ERROR, Message: common_err.GetMsg(common_err.SERVICE_ERROR), Data: err.Error()})
	}

	for i := 0; i < len(r.MountPoints); i++ {
		// Fs may be "name" or "name:remote/path" (e.g. sftp mounts with a chosen remote folder) -
		// config lookups are keyed by the bare remote name only.
		remoteName := strings.SplitN(r.MountPoints[i].Fs, ":", 2)[0]
		dataMap, err := service.MyService.Storage().GetConfigByName(remoteName)
		if err != nil {
			logger.Error("GetConfigByName", zap.Any("err", err))
			continue
		}
		if dataMap["type"] == "drive" {
			r.MountPoints[i].Icon = google_drive.ICONURL
		}
		if dataMap["type"] == "dropbox" {
			r.MountPoints[i].Icon = dropbox.ICONURL
		}
		if dataMap["type"] == "onedrive" {
			r.MountPoints[i].Icon = onedrive.ICONURL
		}
		if dataMap["display_name"] != "" {
			r.MountPoints[i].Name = dataMap["display_name"]
		} else {
			r.MountPoints[i].Name = dataMap["username"]
		}
	}
	list := []httper.MountPoint{}

	for _, v := range r.MountPoints {
		list = append(list, httper.MountPoint(v))
	}

	return ctx.JSON(common_err.SUCCESS, model.Result{Success: common_err.SUCCESS, Message: common_err.GetMsg(common_err.SUCCESS), Data: list})
}

func UmountStorage(ctx echo.Context) error {
	json := make(map[string]string)
	ctx.Bind(&json)
	mountPoint := json["mount_point"]
	if mountPoint == "" {
		return ctx.JSON(common_err.CLIENT_ERROR, model.Result{Success: common_err.CLIENT_ERROR, Message: common_err.GetMsg(common_err.CLIENT_ERROR), Data: "mount_point is empty"})
	}
	err := service.MyService.Storage().UnmountStorage(mountPoint)
	if err != nil {
		return ctx.JSON(common_err.SERVICE_ERROR, model.Result{Success: common_err.SERVICE_ERROR, Message: common_err.GetMsg(common_err.SERVICE_ERROR), Data: err.Error()})
	}
	name := strings.ReplaceAll(mountPoint, "/mnt/", "")
	if cfg, err := service.MyService.Storage().GetConfigByName(name); err == nil {
		if keyFile := cfg["key_file"]; keyFile != "" {
			_ = os.Remove(keyFile)
		}
	}
	service.MyService.Storage().DeleteConfigByName(name)
	return ctx.JSON(common_err.SUCCESS, model.Result{Success: common_err.SUCCESS, Message: common_err.GetMsg(common_err.SUCCESS), Data: "success"})
}

// GetStoragesHealth checks every mounted network/cloud storage by actually listing its remote
// path (not just checking whether rclone still considers it mounted, which doesn't catch a stale
// connection). Checks run concurrently so one slow/hung remote doesn't delay the others.
func GetStoragesHealth(ctx echo.Context) error {
	r, err := service.MyService.Storage().GetStorages()
	if err != nil {
		return ctx.JSON(common_err.SERVICE_ERROR, model.Result{Success: common_err.SERVICE_ERROR, Message: common_err.GetMsg(common_err.SERVICE_ERROR), Data: err.Error()})
	}

	results := make([]StorageHealth, len(r.MountPoints))
	var wg sync.WaitGroup
	for i, mp := range r.MountPoints {
		wg.Add(1)
		go func(i int, mp httper.MountPoints) {
			defer wg.Done()

			remoteName := strings.SplitN(mp.Fs, ":", 2)[0]
			cfg, _ := service.MyService.Storage().GetConfigByName(remoteName)

			start := time.Now()
			testErr := service.MyService.Storage().TestConnection(mp.Fs)
			elapsed := time.Since(start).Milliseconds()

			health := StorageHealth{
				ID:         remoteName,
				MountPoint: mp.MountPoint,
				Name:       firstNonEmpty(cfg["display_name"], cfg["username"], remoteName),
				Type:       cfg["type"],
				Healthy:    testErr == nil,
				LatencyMs:  elapsed,
				Host:       cfg["host"],
				Port:       cfg["port"],
				Username:   cfg["user"],
				RemotePath: cfg["remote_path"],
			}
			if testErr != nil {
				health.Error = testErr.Error()
			}
			results[i] = health
		}(i, mp)
	}
	wg.Wait()

	return ctx.JSON(common_err.SUCCESS, model.Result{Success: common_err.SUCCESS, Message: common_err.GetMsg(common_err.SUCCESS), Data: results})
}

// ReconnectStorage force-unmounts and remounts a storage. Useful when a mount has gone stale
// without rclone itself noticing (see GetStoragesHealth).
func ReconnectStorage(ctx echo.Context) error {
	json := make(map[string]string)
	ctx.Bind(&json)
	mountPoint := json["mount_point"]
	if mountPoint == "" {
		return ctx.JSON(common_err.CLIENT_ERROR, model.Result{Success: common_err.CLIENT_ERROR, Message: common_err.GetMsg(common_err.CLIENT_ERROR), Data: "mount_point is empty"})
	}
	if err := service.MyService.Storage().ReconnectStorage(mountPoint); err != nil {
		return ctx.JSON(common_err.SERVICE_ERROR, model.Result{Success: common_err.SERVICE_ERROR, Message: err.Error()})
	}
	return ctx.JSON(common_err.SUCCESS, model.Result{Success: common_err.SUCCESS, Message: common_err.GetMsg(common_err.SUCCESS)})
}

func GetStorage(ctx echo.Context) error {
	// idStr := ctx.QueryParam("id")
	// id, err := strconv.Atoi(idStr)
	// if err != nil {
	// 	return ctx.JSON(common_err.SUCCESS, model.Result{Success: common_err.CLIENT_ERROR, Message: common_err.GetMsg(common_err.CLIENT_ERROR), Data: err.Error()})
	// 	return
	// }
	// storage, err := service.MyService.Storage().GetStorageById(uint(id))
	// if err != nil {
	// 	return ctx.JSON(common_err.SUCCESS, model.Result{Success: common_err.SERVICE_ERROR, Message: common_err.GetMsg(common_err.SERVICE_ERROR), Data: err.Error()})
	// 	return
	// }
	// return ctx.JSON(common_err.SUCCESS, model.Result{Success: common_err.SUCCESS, Message: common_err.GetMsg(common_err.SUCCESS), Data: storage})
	return nil
}
