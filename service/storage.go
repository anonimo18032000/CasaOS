package service

import (
	"io/ioutil"
	"strings"

	"github.com/IceWhaleTech/CasaOS-Common/utils/logger"
	"github.com/IceWhaleTech/CasaOS/pkg/utils/file"
	"github.com/IceWhaleTech/CasaOS/pkg/utils/httper"
	"go.uber.org/zap"
)

type StorageService interface {
	MountStorage(mountPoint, fs string) error
	UnmountStorage(mountPoint string) error
	GetStorages() (httper.MountList, error)
	CreateConfig(data map[string]string, name string, t string) error
	CreateConfigWithObscure(data map[string]string, name string, t string) error
	UpdateConfig(name string, data map[string]string) error
	TestConnection(fs string) error
	CheckAndMountByName(name string) error
	CheckAndMountAll() error
	ReconnectStorage(mountPoint string) error
	GetConfigByName(name string) (map[string]string, error)
	DeleteConfigByName(name string) error
	GetConfig() (httper.RemotesResult, error)
}

// remoteFs builds the "name:path" fs spec used to mount/remount a remote, honoring a custom
// remote_path if one was stored in its config (e.g. sftp mounts pointed at a specific subfolder).
// Remotes without a stored remote_path (dropbox/drive/onedrive) fall back to "name:" as before.
func remoteFs(name string, cfg map[string]string) string {
	if remotePath := cfg["remote_path"]; remotePath != "" {
		return name + ":" + remotePath
	}
	return name + ":"
}

type storageStruct struct {
}

func (s *storageStruct) MountStorage(mountPoint, fs string) error {
	file.IsNotExistMkDir(mountPoint)
	return httper.Mount(mountPoint, fs)
}
func (s *storageStruct) UnmountStorage(mountPoint string) error {
	err := httper.Unmount(mountPoint)
	if err == nil {
		dir, _ := ioutil.ReadDir(mountPoint)

		if len(dir) == 0 {
			file.RMDir(mountPoint)
		}
		return nil
	}
	return err
}
func (s *storageStruct) GetStorages() (httper.MountList, error) {
	return httper.GetMountList()
}
func (s *storageStruct) CreateConfig(data map[string]string, name string, t string) error {
	httper.CreateConfig(data, name, t)
	return nil
}
func (s *storageStruct) CreateConfigWithObscure(data map[string]string, name string, t string) error {
	return httper.CreateConfigWithObscure(data, name, t)
}
func (s *storageStruct) UpdateConfig(name string, data map[string]string) error {
	return httper.UpdateConfig(name, data)
}
func (s *storageStruct) TestConnection(fs string) error {
	return httper.TestConnection(fs)
}
func (s *storageStruct) CheckAndMountByName(name string) error {
	storages, _ := MyService.Storage().GetStorages()
	currentRemote, _ := httper.GetConfigByName(name)
	mountPoint := currentRemote["mount_point"]
	isMount := false
	for _, v := range storages.MountPoints {
		if v.MountPoint == mountPoint {
			isMount = true
			break
		}
	}
	if !isMount {
		return MyService.Storage().MountStorage(mountPoint, remoteFs(name, currentRemote))
	}
	return nil
}
func (s *storageStruct) CheckAndMountAll() error {
	storages, err := MyService.Storage().GetStorages()
	if err != nil {
		return err
	}
	logger.Info("when CheckAndMountAll storages", zap.Any("storages", storages))
	section, err := httper.GetAllConfigName()
	if err != nil {
		return err
	}
	logger.Info("when CheckAndMountAll section", zap.Any("section", section))
	for _, v := range section.Remotes {
		currentRemote, _ := httper.GetConfigByName(v)
		mountPoint := currentRemote["mount_point"]
		if len(mountPoint) == 0 {
			continue
		}
		isMount := false
		for _, v := range storages.MountPoints {
			if v.MountPoint == mountPoint {
				isMount = true
				break
			}
		}
		if !isMount {
			fs := remoteFs(v, currentRemote)
			logger.Info("when CheckAndMountAll MountStorage", zap.String("mountPoint", mountPoint), zap.String("fs", fs))
			err := MyService.Storage().MountStorage(mountPoint, fs)
			if err != nil {
				logger.Error("when CheckAndMountAll then", zap.String("mountPoint", mountPoint), zap.String("fs", v), zap.Error(err))
			}
		}
	}
	return nil
}

// ReconnectStorage force-unmounts and remounts a storage, honoring its stored remote_path. Useful
// when a mount has gone stale (e.g. the remote server dropped the connection) without CasaOS
// noticing, since a plain "is it in the mount list" check doesn't catch that.
func (s *storageStruct) ReconnectStorage(mountPoint string) error {
	name := strings.TrimPrefix(mountPoint, "/mnt/")
	cfg, err := httper.GetConfigByName(name)
	if err != nil {
		return err
	}
	_ = s.UnmountStorage(mountPoint)
	return s.MountStorage(mountPoint, remoteFs(name, cfg))
}

func (s *storageStruct) GetConfigByName(name string) (map[string]string, error) {
	return httper.GetConfigByName(name)
}
func (s *storageStruct) DeleteConfigByName(name string) error {
	return httper.DeleteConfigByName(name)
}
func (s *storageStruct) GetConfig() (httper.RemotesResult, error) {
	section, err := httper.GetAllConfigName()
	if err != nil {
		return httper.RemotesResult{}, err
	}
	return section, nil
}
func NewStorageService() StorageService {
	return &storageStruct{}
}
