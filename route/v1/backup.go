package v1

import (
	"path/filepath"
	"regexp"
	"strings"

	"github.com/IceWhaleTech/CasaOS-Common/utils/logger"
	"github.com/IceWhaleTech/CasaOS/model"
	"github.com/IceWhaleTech/CasaOS/pkg/utils/common_err"
	"github.com/IceWhaleTech/CasaOS/service"
	"github.com/labstack/echo/v4"
	"github.com/robfig/cron/v3"
	"go.uber.org/zap"
)

// ListBackupJobs returns all configured scheduled backups.
func ListBackupJobs(ctx echo.Context) error {
	jobs := service.MyService.Backup().ListJobs()
	return ctx.JSON(common_err.SUCCESS, model.Result{Success: common_err.SUCCESS, Message: common_err.GetMsg(common_err.SUCCESS), Data: jobs})
}

// backupSourceRoots are the only places a backup may read from: user data and mounted storages.
// Anything else (/etc, /root, /var/lib/casaos, ...) is off limits even for an authenticated user,
// since the copy runs inside rclone as root.
var backupSourceRoots = []string{"/DATA", "/mnt"}

var backupRemoteNamePattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

func isUnderBackupRoot(path string) bool {
	for _, root := range backupSourceRoots {
		if path == root || strings.HasPrefix(path, root+"/") {
			return true
		}
	}
	return false
}

// validateBackupSourcePath cleans path and checks it is inside an allowed root, both literally and
// after resolving symlinks (so /DATA/link -> /etc is rejected). It returns the cleaned path.
func validateBackupSourcePath(path string) (string, error) {
	cleaned := filepath.Clean(path)
	if !filepath.IsAbs(cleaned) || !isUnderBackupRoot(cleaned) {
		return "", &backupValidationError{"source_path must be inside /DATA or /mnt"}
	}

	if resolved, err := filepath.EvalSymlinks(cleaned); err == nil {
		if !isUnderBackupRoot(resolved) {
			// /DATA or /mnt may themselves be symlinks to the real location
			okRoot := false
			for _, root := range backupSourceRoots {
				if realRoot, err := filepath.EvalSymlinks(root); err == nil && (resolved == realRoot || strings.HasPrefix(resolved, realRoot+"/")) {
					okRoot = true
					break
				}
			}
			if !okRoot {
				return "", &backupValidationError{"source_path resolves outside /DATA and /mnt"}
			}
		}
	}

	return cleaned, nil
}

func validateBackupJob(job *service.BackupJob) error {
	if job.Name == "" || job.SourcePath == "" || job.RemoteName == "" {
		return errBackupFieldsRequired
	}

	source, err := validateBackupSourcePath(job.SourcePath)
	if err != nil {
		return err
	}
	job.SourcePath = source

	// the remote name ends up in an rclone "remote:path" spec; rclone also accepts ad-hoc
	// connection strings (":sftp,host=...:") and local paths there, so only plain names of
	// remotes that were configured through CasaOS are allowed.
	if !backupRemoteNamePattern.MatchString(job.RemoteName) {
		return &backupValidationError{"remote_name is not a valid storage name"}
	}
	if cfg, err := service.MyService.Storage().GetConfigByName(job.RemoteName); err != nil || cfg["type"] == "" {
		return &backupValidationError{"remote_name does not match a configured storage"}
	}

	if _, err := cron.ParseStandard(job.Schedule); err != nil {
		return err
	}
	return nil
}

var errBackupFieldsRequired = &backupValidationError{"name, source_path and remote_name are required"}

type backupValidationError struct{ message string }

func (e *backupValidationError) Error() string { return e.message }

// CreateBackupJob adds a new scheduled backup (a one-way copy from a local path to an existing
// configured remote - SFTP, Dropbox, Google Drive, OneDrive, ...).
func CreateBackupJob(ctx echo.Context) error {
	var job service.BackupJob
	if err := ctx.Bind(&job); err != nil {
		return ctx.JSON(common_err.CLIENT_ERROR, model.Result{Success: common_err.CLIENT_ERROR, Message: err.Error()})
	}
	job.RemotePath = normalizeRemotePath(job.RemotePath)

	if err := validateBackupJob(&job); err != nil {
		return ctx.JSON(common_err.CLIENT_ERROR, model.Result{Success: common_err.CLIENT_ERROR, Message: err.Error()})
	}

	created, err := service.MyService.Backup().CreateJob(job)
	if err != nil {
		logger.Error("failed to create backup job", zap.Error(err))
		return ctx.JSON(common_err.SERVICE_ERROR, model.Result{Success: common_err.SERVICE_ERROR, Message: err.Error()})
	}

	return ctx.JSON(common_err.SUCCESS, model.Result{Success: common_err.SUCCESS, Message: common_err.GetMsg(common_err.SUCCESS), Data: created})
}

// EditBackupJob updates an existing backup job's settings and reschedules it.
func EditBackupJob(ctx echo.Context) error {
	var job service.BackupJob
	if err := ctx.Bind(&job); err != nil {
		return ctx.JSON(common_err.CLIENT_ERROR, model.Result{Success: common_err.CLIENT_ERROR, Message: err.Error()})
	}
	job.ID = strings.TrimSpace(ctx.Param("id"))
	job.RemotePath = normalizeRemotePath(job.RemotePath)

	if job.ID == "" {
		return ctx.JSON(common_err.CLIENT_ERROR, model.Result{Success: common_err.CLIENT_ERROR, Message: "id is required"})
	}
	if err := validateBackupJob(&job); err != nil {
		return ctx.JSON(common_err.CLIENT_ERROR, model.Result{Success: common_err.CLIENT_ERROR, Message: err.Error()})
	}

	if err := service.MyService.Backup().UpdateJob(job); err != nil {
		return ctx.JSON(common_err.SERVICE_ERROR, model.Result{Success: common_err.SERVICE_ERROR, Message: err.Error()})
	}

	return ctx.JSON(common_err.SUCCESS, model.Result{Success: common_err.SUCCESS, Message: common_err.GetMsg(common_err.SUCCESS)})
}

// DeleteBackupJob removes a backup job and cancels its schedule.
func DeleteBackupJob(ctx echo.Context) error {
	id := ctx.Param("id")
	if id == "" {
		return ctx.JSON(common_err.CLIENT_ERROR, model.Result{Success: common_err.CLIENT_ERROR, Message: "id is required"})
	}
	if err := service.MyService.Backup().DeleteJob(id); err != nil {
		return ctx.JSON(common_err.SERVICE_ERROR, model.Result{Success: common_err.SERVICE_ERROR, Message: err.Error()})
	}
	return ctx.JSON(common_err.SUCCESS, model.Result{Success: common_err.SUCCESS, Message: common_err.GetMsg(common_err.SUCCESS)})
}

// RunBackupJobNow triggers a backup job immediately, outside its schedule. Starts the job in the
// background and returns right away - a large source can take hours to copy, and holding the HTTP
// request open that long would just time out at the browser or a reverse proxy. Poll
// ListBackupJobs for last_run/last_status to see the result.
func RunBackupJobNow(ctx echo.Context) error {
	id := ctx.Param("id")
	if id == "" {
		return ctx.JSON(common_err.CLIENT_ERROR, model.Result{Success: common_err.CLIENT_ERROR, Message: "id is required"})
	}
	if err := service.MyService.Backup().RunJobAsync(id); err != nil {
		return ctx.JSON(common_err.SERVICE_ERROR, model.Result{Success: common_err.SERVICE_ERROR, Message: err.Error()})
	}
	return ctx.JSON(common_err.SUCCESS, model.Result{Success: common_err.SUCCESS, Message: "backup started"})
}

// GetBackupProgress reports whether a backup job is running and how far along the copy is.
func GetBackupProgress(ctx echo.Context) error {
	progress := service.MyService.Backup().Progress(ctx.Param("id"))
	return ctx.JSON(common_err.SUCCESS, model.Result{Success: common_err.SUCCESS, Message: common_err.GetMsg(common_err.SUCCESS), Data: progress})
}
