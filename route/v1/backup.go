package v1

import (
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

func validateBackupJob(job service.BackupJob) error {
	if job.Name == "" || job.SourcePath == "" || job.RemoteName == "" {
		return errBackupFieldsRequired
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

	if err := validateBackupJob(job); err != nil {
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
	if err := validateBackupJob(job); err != nil {
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

// RunBackupJobNow triggers a backup job immediately, outside its schedule. Runs synchronously -
// callers should expect this to take a while for large sources.
func RunBackupJobNow(ctx echo.Context) error {
	id := ctx.Param("id")
	if id == "" {
		return ctx.JSON(common_err.CLIENT_ERROR, model.Result{Success: common_err.CLIENT_ERROR, Message: "id is required"})
	}
	if err := service.MyService.Backup().RunJob(id); err != nil {
		return ctx.JSON(common_err.SERVICE_ERROR, model.Result{Success: common_err.SERVICE_ERROR, Message: err.Error()})
	}
	return ctx.JSON(common_err.SUCCESS, model.Result{Success: common_err.SUCCESS, Message: common_err.GetMsg(common_err.SUCCESS)})
}
