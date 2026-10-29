package v1

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/IceWhaleTech/CasaOS-Common/utils/constants"
	"github.com/IceWhaleTech/CasaOS/common"
	"github.com/IceWhaleTech/CasaOS/model"
	"github.com/IceWhaleTech/CasaOS/pkg/utils/common_err"
	"github.com/IceWhaleTech/CasaOS/service"
	"github.com/labstack/echo/v4"
)

// Backup and restore of CasaOS' own settings: the *.conf / *.ini files in /etc/casaos and the list
// of scheduled backup jobs. Databases, JWT keys and storage credentials are deliberately NOT
// included - the archive is meant to be safe to keep, and network storages are re-added in the UI.

const (
	configManifestName = "manifest.json"
	configDirPrefix    = "config/"
	backupJobsEntry    = "data/backup_jobs.json"
	maxConfigFileSize  = 2 << 20
	maxConfigUpload    = 10 << 20
)

type configManifest struct {
	Format  int      `json:"format"`
	Created string   `json:"created"`
	Version string   `json:"casaos_version"`
	Files   []string `json:"files"`
}

var configExtensions = map[string]bool{".conf": true, ".ini": true}

// isConfigFileName accepts plain file names only (no directories) with a config extension, and
// refuses anything that looks like key material.
func isConfigFileName(name string) bool {
	if name == "" || name != filepath.Base(name) || strings.ContainsAny(name, `/\`) || strings.HasPrefix(name, ".") {
		return false
	}
	lower := strings.ToLower(name)
	for _, bad := range []string{"key", "secret", "token", "pem", "pass"} {
		if strings.Contains(lower, bad) {
			return false
		}
	}
	return configExtensions[filepath.Ext(lower)]
}

func configTargetPath(entry string) (string, bool) {
	if entry == backupJobsEntry {
		return filepath.Join(constants.DefaultDataPath, "backup_jobs.json"), true
	}
	if strings.HasPrefix(entry, configDirPrefix) {
		name := strings.TrimPrefix(entry, configDirPrefix)
		if isConfigFileName(name) {
			return filepath.Join(constants.DefaultConfigPath, name), true
		}
	}
	return "", false
}

// ExportConfigBackup streams a .tar.gz with the settings.
func ExportConfigBackup(ctx echo.Context) error {
	type item struct {
		entry string
		data  []byte
	}
	var items []item

	if dirEntries, err := os.ReadDir(constants.DefaultConfigPath); err == nil {
		for _, e := range dirEntries {
			if !e.Type().IsRegular() || !isConfigFileName(e.Name()) {
				continue
			}
			data, err := os.ReadFile(filepath.Join(constants.DefaultConfigPath, e.Name()))
			if err != nil || len(data) > maxConfigFileSize {
				continue
			}
			items = append(items, item{configDirPrefix + e.Name(), data})
		}
	}
	if data, err := os.ReadFile(filepath.Join(constants.DefaultDataPath, "backup_jobs.json")); err == nil && len(data) <= maxConfigFileSize {
		items = append(items, item{backupJobsEntry, data})
	}
	if len(items) == 0 {
		return errorJSON(ctx, fmt.Errorf("no configuration files found"))
	}

	manifest := configManifest{Format: 1, Created: time.Now().Format(time.RFC3339), Version: common.VERSION}
	for _, it := range items {
		manifest.Files = append(manifest.Files, it.entry)
	}
	manifestData, _ := json.MarshalIndent(manifest, "", "  ")

	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	write := func(name string, data []byte) error {
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o600, Size: int64(len(data)), ModTime: time.Now(), Typeflag: tar.TypeReg}); err != nil {
			return err
		}
		_, err := tw.Write(data)
		return err
	}
	if err := write(configManifestName, manifestData); err != nil {
		return errorJSON(ctx, err)
	}
	for _, it := range items {
		if err := write(it.entry, it.data); err != nil {
			return errorJSON(ctx, err)
		}
	}
	if err := tw.Close(); err != nil {
		return errorJSON(ctx, err)
	}
	if err := gz.Close(); err != nil {
		return errorJSON(ctx, err)
	}

	name := "casaos-config-" + time.Now().Format("20060102-150405") + ".tar.gz"
	ctx.Response().Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
	ctx.Response().Header().Set("Cache-Control", "no-store")
	return ctx.Blob(http.StatusOK, "application/gzip", buf.Bytes())
}

// readConfigArchive parses and validates an uploaded archive without touching the disk.
func readConfigArchive(r io.Reader) (map[string][]byte, error) {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return nil, fmt.Errorf("not a valid backup file")
	}
	defer gz.Close()

	files := map[string][]byte{}
	var total int64
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("the backup file is damaged")
		}
		if hdr.Typeflag != tar.TypeReg {
			return nil, fmt.Errorf("unexpected entry %q in the backup file", hdr.Name)
		}
		name := path.Clean(hdr.Name)
		if name != configManifestName {
			if _, ok := configTargetPath(name); !ok {
				return nil, fmt.Errorf("the backup contains an unexpected file: %q", hdr.Name)
			}
		}
		if _, dup := files[name]; dup {
			return nil, fmt.Errorf("the backup lists %q twice", name)
		}
		if hdr.Size > maxConfigFileSize {
			return nil, fmt.Errorf("%q is too large", name)
		}
		data, err := io.ReadAll(io.LimitReader(tr, maxConfigFileSize+1))
		if err != nil || int64(len(data)) > maxConfigFileSize {
			return nil, fmt.Errorf("%q could not be read", name)
		}
		if total += int64(len(data)); total > maxConfigUpload {
			return nil, fmt.Errorf("the backup is too large")
		}
		files[name] = data
	}

	rawManifest, ok := files[configManifestName]
	if !ok {
		return nil, fmt.Errorf("this is not a CasaOS configuration backup (manifest missing)")
	}
	var manifest configManifest
	if err := json.Unmarshal(rawManifest, &manifest); err != nil || manifest.Format != 1 {
		return nil, fmt.Errorf("unsupported backup format")
	}
	delete(files, configManifestName)

	if data, ok := files[backupJobsEntry]; ok {
		var jobs []service.BackupJob
		if err := json.Unmarshal(data, &jobs); err != nil {
			return nil, fmt.Errorf("the backup jobs list in the file is invalid")
		}
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("the backup is empty")
	}
	return files, nil
}

// writeKeepingOld replaces target atomically, first copying the current file to <target>.pre-restore.
func writeKeepingOld(target string, data []byte) error {
	mode := os.FileMode(0o644)
	if info, err := os.Stat(target); err == nil {
		mode = info.Mode().Perm()
		if old, err := os.ReadFile(target); err == nil {
			if err := os.WriteFile(target+".pre-restore", old, 0o600); err != nil {
				return err
			}
		}
	}
	tmp := target + ".tmp"
	if err := os.WriteFile(tmp, data, mode); err != nil {
		return err
	}
	if err := os.Rename(tmp, target); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

// ImportConfigBackup restores the files of an archive made by ExportConfigBackup. It validates the
// whole archive first and only then writes; every file it replaces is kept as <name>.pre-restore.
// Services are not restarted - the response says so and the UI tells the user.
func ImportConfigBackup(ctx echo.Context) error {
	ctx.Request().Body = http.MaxBytesReader(ctx.Response(), ctx.Request().Body, maxConfigUpload+(1<<20))
	fh, err := ctx.FormFile("file")
	if err != nil {
		return errorJSON(ctx, requestError("upload the backup file in the 'file' field"))
	}
	src, err := fh.Open()
	if err != nil {
		return errorJSON(ctx, err)
	}
	defer src.Close()

	files, err := readConfigArchive(src)
	if err != nil {
		return errorJSON(ctx, requestError(err.Error()))
	}

	restored := []string{}
	for entry, data := range files {
		target, _ := configTargetPath(entry) // already validated by readConfigArchive
		if err := writeKeepingOld(target, data); err != nil {
			return ctx.JSON(common_err.SERVICE_ERROR, model.Result{
				Success: common_err.SERVICE_ERROR, Message: common_err.GetMsg(common_err.SERVICE_ERROR),
				Data: fmt.Sprintf("restored %d file(s), then failed on %s: %v", len(restored), entry, err),
			})
		}
		restored = append(restored, entry)
	}
	return shareOK(ctx, map[string]interface{}{"restored": restored, "restart_required": true})
}
