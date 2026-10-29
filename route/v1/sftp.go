package v1

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/IceWhaleTech/CasaOS-Common/utils/constants"
	"github.com/IceWhaleTech/CasaOS-Common/utils/logger"
	"github.com/IceWhaleTech/CasaOS/model"
	"github.com/IceWhaleTech/CasaOS/pkg/utils/common_err"
	"github.com/IceWhaleTech/CasaOS/service"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

type AddSFTPStorageRequest struct {
	Name       string `json:"name"`
	Host       string `json:"host"`
	Port       string `json:"port"`
	Username   string `json:"username"`
	Password   string `json:"password"`
	PrivateKey string `json:"private_key"`
	RemotePath string `json:"remote_path"`
	// HostKeyFingerprint is optional ("SHA256:..."). When set, the server must present a host key
	// with this fingerprint; when empty, the key seen on this first connection is pinned.
	HostKeyFingerprint string `json:"host_key_fingerprint"`
}

type EditSFTPStorageRequest struct {
	ID          string `json:"id"` // internal remote name to edit (the mount list's "fs")
	DisplayName string `json:"display_name"`
	Host        string `json:"host"`
	Port        string `json:"port"`
	Username    string `json:"username"`
	Password    string `json:"password"`
	PrivateKey  string `json:"private_key"`
	RemotePath  string `json:"remote_path"`

	HostKeyFingerprint string `json:"host_key_fingerprint"`
}

var storageNameSanitizer = regexp.MustCompile(`[^a-zA-Z0-9_-]+`)

var sftpHostPattern = regexp.MustCompile(`^[A-Za-z0-9._:%-]+$`)

// sftpKeysDir returns the directory where uploaded SFTP private keys are stored.
//
// rclone's `key_pem` option (passing the key inline via its RC API) reliably fails with
// "pem key not formatted properly: invalid syntax" regardless of PEM format (confirmed against
// a real server with both OpenSSH and classic RSA PEM formats) - but `key_file` (a path) works
// correctly, so private keys are written to disk instead of passed inline.
func sftpKeysDir() string {
	return filepath.Join(constants.DefaultDataPath, "sftp_keys")
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func normalizeRemotePath(remotePath string) string {
	if remotePath == "" {
		return "/"
	}
	if !strings.HasPrefix(remotePath, "/") {
		return "/" + remotePath
	}
	return remotePath
}

func validateSFTPRequest(host, username, password, privateKey string) error {
	if host == "" || username == "" || (password == "" && privateKey == "") {
		return errors.New("host, username and either password or private_key are required")
	}
	return nil
}

// validateSFTPTarget checks host and port are plausible; both end up in rclone config and in a
// known_hosts file, so stray whitespace/newlines must not get through.
func validateSFTPTarget(host, port string) error {
	if !sftpHostPattern.MatchString(host) {
		return errors.New("host contains invalid characters")
	}
	if n, err := strconv.Atoi(port); err != nil || n < 1 || n > 65535 {
		return errors.New("port must be a number between 1 and 65535")
	}
	return nil
}

// uniqueRemoteName returns base, or base_2, base_3, ... so that a new storage never overwrites the
// config of an existing remote (different display names can sanitize to the same name).
func uniqueRemoteName(base string) string {
	candidate := base
	for i := 2; i < 1000; i++ {
		cfg, err := service.MyService.Storage().GetConfigByName(candidate)
		if err != nil || cfg["type"] == "" {
			return candidate
		}
		candidate = base + "_" + strconv.Itoa(i)
	}
	return candidate
}

func knownHostsFilePath(name string) string {
	return filepath.Join(sftpKnownHostsDir(), filepath.Base(name))
}

// writeSFTPKeyFile writes a private key to dir/filename with restrictive permissions.
func writeSFTPKeyFile(dir, filename, privateKey string) (string, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	path := filepath.Join(dir, filepath.Base(filename))
	if err := os.WriteFile(path, []byte(privateKey), 0o600); err != nil {
		return "", err
	}
	return path, nil
}

// AddSFTPStorage connects to a remote server over SFTP (via rclone) and mounts it under /mnt,
// the same way Google Drive/Dropbox/OneDrive are mounted - just with direct credentials instead
// of an OAuth handshake.
func AddSFTPStorage(ctx echo.Context) error {
	var request AddSFTPStorageRequest
	if err := ctx.Bind(&request); err != nil {
		return ctx.JSON(common_err.CLIENT_ERROR, model.Result{Success: common_err.CLIENT_ERROR, Message: err.Error()})
	}

	port := firstNonEmpty(request.Port, "22")
	if err := validateSFTPRequest(request.Host, request.Username, request.Password, request.PrivateKey); err != nil {
		return ctx.JSON(common_err.CLIENT_ERROR, model.Result{Success: common_err.CLIENT_ERROR, Message: err.Error()})
	}
	if err := validateSFTPTarget(request.Host, port); err != nil {
		return ctx.JSON(common_err.CLIENT_ERROR, model.Result{Success: common_err.CLIENT_ERROR, Message: err.Error()})
	}

	displayName := strings.TrimSpace(request.Name)

	name := storageNameSanitizer.ReplaceAllString(displayName, "_")
	name = strings.Trim(name, "_")
	if name == "" {
		name = "sftp_" + strconv.FormatInt(time.Now().Unix(), 10)
	}
	name = uniqueRemoteName(name)
	if displayName == "" {
		displayName = request.Username
	}

	mountPoint := "/mnt/" + name
	remotePath := normalizeRemotePath(request.RemotePath)

	knownHostsPath := knownHostsFilePath(name)
	if _, err := pinHostKeys(request.Host, port, request.HostKeyFingerprint, knownHostsPath); err != nil {
		return ctx.JSON(common_err.CLIENT_ERROR, model.Result{Success: common_err.CLIENT_ERROR, Message: err.Error()})
	}

	dmap := map[string]string{
		"host":             request.Host,
		"port":             port,
		"user":             request.Username,
		"type":             "sftp",
		"mount_point":      mountPoint,
		"username":         request.Username,
		"remote_path":      remotePath,
		"display_name":     displayName,
		"known_hosts_file": knownHostsPath,
	}

	var keyFilePath string
	if request.PrivateKey != "" {
		var err error
		keyFilePath, err = writeSFTPKeyFile(sftpKeysDir(), name, request.PrivateKey)
		if err != nil {
			logger.Error("failed to write sftp private key", zap.Error(err), zap.String("name", name))
			_ = os.Remove(knownHostsPath)
			return ctx.JSON(common_err.SERVICE_ERROR, model.Result{Success: common_err.SERVICE_ERROR, Message: err.Error()})
		}
		dmap["key_file"] = keyFilePath
	} else {
		dmap["pass"] = request.Password
	}

	cleanup := func() {
		_ = os.Remove(knownHostsPath)
		if keyFilePath != "" {
			_ = os.Remove(keyFilePath)
		}
	}

	if err := service.MyService.Storage().CreateConfigWithObscure(dmap, name, "sftp"); err != nil {
		logger.Error("failed to create sftp config", zap.Error(err), zap.String("name", name))
		cleanup()
		return ctx.JSON(common_err.SERVICE_ERROR, model.Result{Success: common_err.SERVICE_ERROR, Message: err.Error()})
	}

	if err := service.MyService.Storage().MountStorage(mountPoint, name+":"+remotePath); err != nil {
		logger.Error("failed to mount sftp storage", zap.Error(err), zap.String("name", name), zap.String("mountPoint", mountPoint))
		_ = service.MyService.Storage().DeleteConfigByName(name)
		cleanup()
		return ctx.JSON(common_err.SERVICE_ERROR, model.Result{Success: common_err.SERVICE_ERROR, Message: err.Error()})
	}

	return ctx.JSON(common_err.SUCCESS, model.Result{
		Success: common_err.SUCCESS,
		Message: common_err.GetMsg(common_err.SUCCESS),
		Data:    mountPoint,
	})
}

// TestSFTPStorage validates SFTP credentials by listing the remote path through a throwaway
// rclone config, without creating a mount or persisting anything. The response carries the host
// key fingerprints the server presented, so the user can compare them with the expected ones.
func TestSFTPStorage(ctx echo.Context) error {
	var request AddSFTPStorageRequest
	if err := ctx.Bind(&request); err != nil {
		return ctx.JSON(common_err.CLIENT_ERROR, model.Result{Success: common_err.CLIENT_ERROR, Message: err.Error()})
	}

	port := firstNonEmpty(request.Port, "22")
	if err := validateSFTPRequest(request.Host, request.Username, request.Password, request.PrivateKey); err != nil {
		return ctx.JSON(common_err.CLIENT_ERROR, model.Result{Success: common_err.CLIENT_ERROR, Message: err.Error()})
	}
	if err := validateSFTPTarget(request.Host, port); err != nil {
		return ctx.JSON(common_err.CLIENT_ERROR, model.Result{Success: common_err.CLIENT_ERROR, Message: err.Error()})
	}

	remotePath := normalizeRemotePath(request.RemotePath)
	testName := "test_" + strconv.FormatInt(time.Now().UnixNano(), 10)

	tempKnownHosts := filepath.Join(os.TempDir(), testName+".known_hosts")
	defer os.Remove(tempKnownHosts)
	fingerprints, err := pinHostKeys(request.Host, port, request.HostKeyFingerprint, tempKnownHosts)
	if err != nil {
		return ctx.JSON(common_err.CLIENT_ERROR, model.Result{
			Success: common_err.CLIENT_ERROR,
			Message: err.Error(),
			Data:    map[string]interface{}{"host_key_fingerprints": fingerprints},
		})
	}

	dmap := map[string]string{
		"host":             request.Host,
		"port":             port,
		"user":             request.Username,
		"type":             "sftp",
		"known_hosts_file": tempKnownHosts,
	}

	if request.PrivateKey != "" {
		tempKeyFile, err := writeSFTPKeyFile(os.TempDir(), testName, request.PrivateKey)
		if err != nil {
			return ctx.JSON(common_err.SERVICE_ERROR, model.Result{Success: common_err.SERVICE_ERROR, Message: err.Error()})
		}
		defer os.Remove(tempKeyFile)
		dmap["key_file"] = tempKeyFile
	} else {
		dmap["pass"] = request.Password
	}

	if err := service.MyService.Storage().CreateConfigWithObscure(dmap, testName, "sftp"); err != nil {
		return ctx.JSON(common_err.SERVICE_ERROR, model.Result{Success: common_err.SERVICE_ERROR, Message: err.Error()})
	}
	defer service.MyService.Storage().DeleteConfigByName(testName)

	if err := service.MyService.Storage().TestConnection(testName + ":" + remotePath); err != nil {
		return ctx.JSON(common_err.CLIENT_ERROR, model.Result{Success: common_err.CLIENT_ERROR, Message: err.Error()})
	}

	return ctx.JSON(common_err.SUCCESS, model.Result{
		Success: common_err.SUCCESS,
		Message: "connection successful",
		Data:    map[string]interface{}{"host_key_fingerprints": fingerprints},
	})
}

// EditSFTPStorage updates an existing SFTP connection's settings (display name, host, port,
// username, credentials, or remote folder). The underlying rclone remote name and /mnt mount
// point stay fixed - only what they point at can change. Whenever anything other than the
// display name changes, the mount is torn down and rebuilt so the new settings take effect.
// If rebuilding fails, the previous configuration is restored so the storage isn't lost.
func EditSFTPStorage(ctx echo.Context) error {
	var request EditSFTPStorageRequest
	if err := ctx.Bind(&request); err != nil {
		return ctx.JSON(common_err.CLIENT_ERROR, model.Result{Success: common_err.CLIENT_ERROR, Message: err.Error()})
	}
	if request.ID == "" {
		return ctx.JSON(common_err.CLIENT_ERROR, model.Result{Success: common_err.CLIENT_ERROR, Message: "id is required"})
	}

	current, err := service.MyService.Storage().GetConfigByName(request.ID)
	if err != nil || current["type"] != "sftp" {
		return ctx.JSON(common_err.CLIENT_ERROR, model.Result{Success: common_err.CLIENT_ERROR, Message: "sftp storage not found"})
	}

	mountPoint := current["mount_point"]
	host := firstNonEmpty(request.Host, current["host"])
	port := firstNonEmpty(request.Port, current["port"], "22")
	username := firstNonEmpty(request.Username, current["user"])
	displayName := firstNonEmpty(strings.TrimSpace(request.DisplayName), current["display_name"], username)
	remotePath := normalizeRemotePath(firstNonEmpty(request.RemotePath, current["remote_path"]))
	oldRemotePath := normalizeRemotePath(current["remote_path"])

	if err := validateSFTPTarget(host, port); err != nil {
		return ctx.JSON(common_err.CLIENT_ERROR, model.Result{Success: common_err.CLIENT_ERROR, Message: err.Error()})
	}

	changingSecret := request.Password != "" || request.PrivateKey != ""
	hostChanged := host != current["host"] || port != current["port"]
	needsRemount := changingSecret ||
		remotePath != oldRemotePath ||
		hostChanged || username != current["user"]

	// Pin the host key again when the server changed, when the user gives a fingerprint to check,
	// or when this storage predates host key checking and is being rebuilt anyway.
	repin := request.HostKeyFingerprint != "" || hostChanged || (current["known_hosts_file"] == "" && needsRemount)
	knownHostsPath := current["known_hosts_file"]
	if repin {
		knownHostsPath = knownHostsFilePath(request.ID)
		if _, err := pinHostKeys(host, port, request.HostKeyFingerprint, knownHostsPath); err != nil {
			return ctx.JSON(common_err.CLIENT_ERROR, model.Result{Success: common_err.CLIENT_ERROR, Message: err.Error()})
		}
		needsRemount = true
	}

	dmap := map[string]string{
		"host":         host,
		"port":         port,
		"user":         username,
		"mount_point":  mountPoint,
		"username":     username,
		"remote_path":  remotePath,
		"display_name": displayName,
	}
	if knownHostsPath != "" {
		dmap["known_hosts_file"] = knownHostsPath
	}

	// snapshot of the stored config (passwords are already obscured in it) to roll back to
	previous := make(map[string]string, len(current))
	for k, v := range current {
		previous[k] = v
	}

	oldKeyFile := current["key_file"]
	var newKeyFile string
	if changingSecret {
		if request.PrivateKey != "" {
			// a new file name, so the old key is still there if we have to roll back
			keyName := request.ID + "_" + strconv.FormatInt(time.Now().UnixNano(), 10)
			newKeyFile, err = writeSFTPKeyFile(sftpKeysDir(), keyName, request.PrivateKey)
			if err != nil {
				return ctx.JSON(common_err.SERVICE_ERROR, model.Result{Success: common_err.SERVICE_ERROR, Message: err.Error()})
			}
			dmap["key_file"] = newKeyFile
		} else {
			dmap["pass"] = request.Password
		}
	}

	rollback := func() {
		if newKeyFile != "" {
			_ = os.Remove(newKeyFile)
		}
		_ = service.MyService.Storage().DeleteConfigByName(request.ID)
		if err := service.MyService.Storage().CreateConfig(previous, request.ID, "sftp"); err != nil {
			logger.Error("failed to restore sftp config", zap.Error(err), zap.String("name", request.ID))
			return
		}
		if needsRemount {
			if err := service.MyService.Storage().MountStorage(mountPoint, request.ID+":"+oldRemotePath); err != nil {
				logger.Error("failed to remount previous sftp storage", zap.Error(err), zap.String("name", request.ID))
			}
		}
	}

	if needsRemount {
		_ = service.MyService.Storage().UnmountStorage(mountPoint)
	}

	if changingSecret {
		// Switching between password and key auth (or rotating either) needs a clean slate -
		// config/update only merges fields, so a stale key_file/pass could linger otherwise.
		_ = service.MyService.Storage().DeleteConfigByName(request.ID)
		dmap["type"] = "sftp"
		if err := service.MyService.Storage().CreateConfigWithObscure(dmap, request.ID, "sftp"); err != nil {
			logger.Error("failed to recreate sftp config", zap.Error(err), zap.String("name", request.ID))
			rollback()
			return ctx.JSON(common_err.SERVICE_ERROR, model.Result{Success: common_err.SERVICE_ERROR, Message: err.Error()})
		}
	} else if err := service.MyService.Storage().UpdateConfig(request.ID, dmap); err != nil {
		logger.Error("failed to update sftp config", zap.Error(err), zap.String("name", request.ID))
		rollback()
		return ctx.JSON(common_err.SERVICE_ERROR, model.Result{Success: common_err.SERVICE_ERROR, Message: err.Error()})
	}

	if needsRemount {
		if err := service.MyService.Storage().MountStorage(mountPoint, request.ID+":"+remotePath); err != nil {
			logger.Error("failed to remount sftp storage", zap.Error(err), zap.String("name", request.ID))
			rollback()
			return ctx.JSON(common_err.SERVICE_ERROR, model.Result{Success: common_err.SERVICE_ERROR, Message: err.Error()})
		}
	}

	// the old key is obsolete whenever the secret changed: a new key replaced it, or the storage
	// switched to a password
	if changingSecret && oldKeyFile != "" && oldKeyFile != newKeyFile {
		_ = os.Remove(oldKeyFile)
	}

	return ctx.JSON(common_err.SUCCESS, model.Result{
		Success: common_err.SUCCESS,
		Message: common_err.GetMsg(common_err.SUCCESS),
		Data:    mountPoint,
	})
}
