package httper

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/IceWhaleTech/CasaOS-Common/utils/logger"
	"github.com/go-resty/resty/v2"
	"go.uber.org/zap"
)

type MountList struct {
	MountPoints []MountPoints `json:"mountPoints"`
}
type MountPoints struct {
	MountPoint string `json:"MountPoint"`
	Fs         string `json:"Fs"`
	Icon       string `json:"Icon"`
	Name       string `json:"Name"`
}
type MountPoint struct {
	MountPoint string `json:"mount_point"`
	Fs         string `json:"fs"`
	Icon       string `json:"icon"`
	Name       string `json:"name"`
}
type MountResult struct {
	Error string `json:"error"`
	Input struct {
		Fs         string `json:"fs"`
		MountPoint string `json:"mountPoint"`
	} `json:"input"`
	Path   string `json:"path"`
	Status int    `json:"status"`
}

type RemotesResult struct {
	Remotes []string `json:"remotes"`
}

var UserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/87.0.4280.88 Safari/537.36"
var DefaultTimeout = time.Second * 30

func NewRestyClient() *resty.Client {

	unixSocket := "/var/run/rclone/rclone.sock"

	transport := http.Transport{
		Dial: func(_, _ string) (net.Conn, error) {
			return net.Dial("unix", unixSocket)
		},
	}

	client := resty.New()

	client.SetTransport(&transport).SetBaseURL("http://localhost")
	client.SetRetryCount(3).SetRetryWaitTime(5*time.Second).SetTimeout(DefaultTimeout).SetHeader("User-Agent", UserAgent)
	return client
}

// NewHealthCheckRestyClient is like NewRestyClient but with a short timeout and no retries, for
// checks that need a fast yes/no (connection tests, health dashboards) rather than the patient
// retry behaviour appropriate for an actual mount/create operation.
func NewHealthCheckRestyClient() *resty.Client {
	unixSocket := "/var/run/rclone/rclone.sock"

	transport := http.Transport{
		Dial: func(_, _ string) (net.Conn, error) {
			return net.Dial("unix", unixSocket)
		},
	}

	client := resty.New()

	client.SetTransport(&transport).SetBaseURL("http://localhost")
	client.SetRetryCount(0).SetTimeout(8 * time.Second).SetHeader("User-Agent", UserAgent)
	return client
}

func GetMountList() (MountList, error) {
	var result MountList
	res, err := NewRestyClient().R().Post("/mount/listmounts")
	if err != nil {
		return result, err
	}
	if res.StatusCode() != 200 {
		return result, fmt.Errorf("get mount list failed")
	}
	json.Unmarshal(res.Body(), &result)
	for i := 0; i < len(result.MountPoints); i++ {
		result.MountPoints[i].Fs = result.MountPoints[i].Fs[:len(result.MountPoints[i].Fs)-1]
	}
	return result, err
}

func Mount(mountPoint string, fs string) error {
	res, err := NewRestyClient().R().SetFormData(map[string]string{
		"mountPoint": mountPoint,
		"fs":         fs,
		"mountOpt":   `{"AllowOther": true}`,
		"vfsOpt":     `{"CacheMode": 3}`,
	}).Post("/mount/mount")
	if err != nil {
		return err
	}
	if res.StatusCode() != 200 {
		return fmt.Errorf("mount failed")
	}
	logger.Info("mount then", zap.Any("res", res.Body()))
	return nil
}
func Unmount(mountPoint string) error {
	res, err := NewRestyClient().R().SetFormData(map[string]string{
		"mountPoint": mountPoint,
	}).Post("/mount/unmount")
	if err != nil {
		logger.Error("when unmount", zap.Error(err))
		return err
	}
	if res.StatusCode() != 200 {
		logger.Error("then unmount failed", zap.Any("res", res.Body()))
		return fmt.Errorf("unmount failed")
	}

	logger.Info("unmount then", zap.Any("res", res.Body()))
	return nil
}

func CreateConfig(data map[string]string, name, t string) error {
	data["config_is_local"] = "false"
	dataStr, _ := json.Marshal(data)
	res, err := NewRestyClient().R().SetFormData(map[string]string{
		"name":       name,
		"parameters": string(dataStr),
		"type":       t,
	}).Post("/config/create")
	if err != nil {
		return err
	}
	logger.Info("when create config then", zap.Any("res", res.Body()))
	if res.StatusCode() != 200 {
		return fmt.Errorf("create config failed")
	}

	return nil
}

// CreateConfigWithObscure is like CreateConfig, but tells rclone to obscure fields such as `pass`
// (rclone's own lightweight reversible encoding, not real encryption) before storing them - needed
// for backends like sftp where the password/private key is passed in cleartext by the caller.
func CreateConfigWithObscure(data map[string]string, name, t string) error {
	data["config_is_local"] = "false"
	dataStr, _ := json.Marshal(data)
	res, err := NewRestyClient().R().SetFormData(map[string]string{
		"name":       name,
		"parameters": string(dataStr),
		"type":       t,
		"opt":        `{"obscure": true}`,
	}).Post("/config/create")
	if err != nil {
		return err
	}
	if res.StatusCode() != 200 {
		return fmt.Errorf("create config failed: %s", res.Body())
	}

	return nil
}

// TestConnection checks that fs (a remote, optionally with a path e.g. "name:/path") is reachable
// and listable, without creating a mount. Used to validate credentials before committing to a mount.
func TestConnection(fs string) error {
	// LowLevelRetries=1: with rclone's default (10) a wrong password/host takes ~25s to fail, longer
	// than this client's timeout, so the caller would only ever see "context deadline exceeded"
	// instead of the real error.
	res, err := NewHealthCheckRestyClient().R().SetFormData(map[string]string{
		"fs":      fs,
		"remote":  "",
		"_config": `{"LowLevelRetries":1}`,
	}).Post("/operations/list")
	if err != nil {
		return err
	}
	if res.StatusCode() != 200 {
		var result map[string]interface{}
		_ = json.Unmarshal(res.Body(), &result)
		if errMsg, ok := result["error"].(string); ok {
			return fmt.Errorf("%s", errMsg)
		}
		return fmt.Errorf("connection test failed")
	}
	return nil
}

// UpdateConfig updates an existing remote's parameters in place (e.g. changing host/credentials),
// obscuring password-like fields the same way CreateConfigWithObscure does.
func UpdateConfig(name string, data map[string]string) error {
	dataStr, _ := json.Marshal(data)
	res, err := NewRestyClient().R().SetFormData(map[string]string{
		"name":       name,
		"parameters": string(dataStr),
		"opt":        `{"obscure": true}`,
	}).Post("/config/update")
	if err != nil {
		return err
	}
	if res.StatusCode() != 200 {
		return fmt.Errorf("update config failed: %s", res.Body())
	}
	return nil
}

func GetConfigByName(name string) (map[string]string, error) {

	res, err := NewRestyClient().R().SetFormData(map[string]string{
		"name": name,
	}).Post("/config/get")
	if err != nil {
		return nil, err
	}
	if res.StatusCode() != 200 {
		return nil, fmt.Errorf("create config failed")
	}
	var result map[string]string
	json.Unmarshal(res.Body(), &result)
	return result, nil
}
func GetAllConfigName() (RemotesResult, error) {
	var result RemotesResult
	res, err := NewRestyClient().R().SetFormData(map[string]string{}).Post("/config/listremotes")
	if err != nil {
		return result, err
	}
	if res.StatusCode() != 200 {
		return result, fmt.Errorf("get config failed")
	}

	json.Unmarshal(res.Body(), &result)
	return result, nil
}
func DeleteConfigByName(name string) error {
	res, err := NewRestyClient().R().SetFormData(map[string]string{
		"name": name,
	}).Post("/config/delete")
	if err != nil {
		return err
	}
	if res.StatusCode() != 200 {
		return fmt.Errorf("delete config failed")
	}
	return nil
}

// SyncCopy one-way copies everything from srcPath (a local filesystem path) to dstFs (an
// "remote:path" fs spec), adding/updating files at the destination without deleting anything
// there that isn't in the source - the safer choice for a backup job, as opposed to rclone's
// `sync/sync` which mirrors exactly and would delete destination files removed from the source.
// Backups can run long, so this uses its own client with a much longer timeout than the other
// rclone RC calls (which are all interactive, user-facing requests).
func SyncCopy(srcPath string, dstFs string) error {
	return SyncCopyGroup(srcPath, dstFs, "")
}

// SyncCopyGroup is SyncCopy with the transfer tagged as a rclone stats group, so that its progress
// can be read back with GetGroupStats while it runs. An empty group means "no group".
func SyncCopyGroup(srcPath string, dstFs string, group string) error {
	client := newRcloneClient(4 * time.Hour)

	form := map[string]string{
		"srcFs": srcPath,
		"dstFs": dstFs,
	}
	if group != "" {
		form["_group"] = group
	}
	res, err := client.R().SetFormData(form).Post("/sync/copy")
	if err != nil {
		return err
	}
	if res.StatusCode() != 200 {
		var result map[string]interface{}
		_ = json.Unmarshal(res.Body(), &result)
		if errMsg, ok := result["error"].(string); ok {
			return fmt.Errorf("%s", errMsg)
		}
		return fmt.Errorf("backup copy failed")
	}
	return nil
}

func newRcloneClient(timeout time.Duration) *resty.Client {
	unixSocket := "/var/run/rclone/rclone.sock"

	transport := http.Transport{
		Dial: func(_, _ string) (net.Conn, error) {
			return net.Dial("unix", unixSocket)
		},
	}

	client := resty.New()
	client.SetTransport(&transport).SetBaseURL("http://localhost")
	client.SetRetryCount(0).SetTimeout(timeout).SetHeader("User-Agent", UserAgent)
	return client
}

// GroupStats is the subset of rclone's core/stats that the UI shows as backup progress.
type GroupStats struct {
	Bytes          int64   `json:"bytes"`
	TotalBytes     int64   `json:"totalBytes"`
	Speed          float64 `json:"speed"`
	ETA            *int64  `json:"eta"`
	Transfers      int64   `json:"transfers"`
	TotalTransfers int64   `json:"totalTransfers"`
	Checks         int64   `json:"checks"`
	Errors         int64   `json:"errors"`
	LastError      string  `json:"lastError"`
	Transferring   []struct {
		Name       string  `json:"name"`
		Size       int64   `json:"size"`
		Bytes      int64   `json:"bytes"`
		Percentage int     `json:"percentage"`
		Speed      float64 `json:"speed"`
	} `json:"transferring"`
}

// GetGroupStats reads the live statistics of a transfer started with SyncCopyGroup.
func GetGroupStats(group string) (GroupStats, error) {
	var stats GroupStats
	res, err := newRcloneClient(10*time.Second).R().SetFormData(map[string]string{"group": group}).Post("/core/stats")
	if err != nil {
		return stats, err
	}
	if res.StatusCode() != 200 {
		return stats, fmt.Errorf("core/stats failed: %s", res.Status())
	}
	if err := json.Unmarshal(res.Body(), &stats); err != nil {
		return stats, err
	}
	return stats, nil
}

// ResetGroupStats forgets the statistics of a group, so the next run starts from zero (and rclone
// does not keep one stats group per job forever).
func ResetGroupStats(group string) {
	_, _ = newRcloneClient(10*time.Second).R().SetFormData(map[string]string{"group": group}).Post("/core/stats-delete")
}
