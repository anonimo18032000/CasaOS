package service

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/IceWhaleTech/CasaOS-Common/utils/constants"
	"github.com/IceWhaleTech/CasaOS-Common/utils/logger"
	"github.com/IceWhaleTech/CasaOS/pkg/utils/httper"
	"github.com/google/uuid"
	"github.com/robfig/cron/v3"
	"go.uber.org/zap"
)

// BackupJob describes a scheduled one-way copy from a local path to a remote (any rclone remote
// already configured via the Storage service - SFTP, Dropbox, Google Drive, OneDrive, ...).
type BackupJob struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	SourcePath string `json:"source_path"`
	RemoteName string `json:"remote_name"`
	RemotePath string `json:"remote_path"`
	Schedule   string `json:"schedule"` // standard 5-field cron expression, e.g. "0 2 * * *"
	Enabled    bool   `json:"enabled"`
	LastRun    string `json:"last_run,omitempty"`
	LastStatus string `json:"last_status,omitempty"` // "success" | "error"
	LastError  string `json:"last_error,omitempty"`
}

type BackupService interface {
	ListJobs() []BackupJob
	CreateJob(job BackupJob) (BackupJob, error)
	UpdateJob(job BackupJob) error
	DeleteJob(id string) error
	RunJob(id string) error
	RunJobAsync(id string) error
}

func backupJobsFilePath() string {
	return filepath.Join(constants.DefaultDataPath, "backup_jobs.json")
}

type backupStruct struct {
	mu       sync.Mutex
	jobs     []BackupJob
	cronJob  *cron.Cron
	entryIDs map[string]cron.EntryID
	running  map[string]bool
}

// NewBackupService loads persisted backup jobs from disk and starts their cron schedules. Copies
// use rclone's `sync/copy` (one-way, additive) rather than `sync/sync`, since a backup shouldn't
// delete files at the destination just because they were removed from the source.
func NewBackupService() BackupService {
	s := &backupStruct{
		cronJob:  cron.New(),
		entryIDs: make(map[string]cron.EntryID),
		running:  make(map[string]bool),
	}
	s.load()
	s.cronJob.Start()
	s.rescheduleAll()
	return s
}

func (s *backupStruct) load() {
	data, err := os.ReadFile(backupJobsFilePath())
	if err != nil {
		s.jobs = []BackupJob{}
		return
	}
	if err := json.Unmarshal(data, &s.jobs); err != nil {
		logger.Error("failed to parse backup jobs file", zap.Error(err))
		s.jobs = []BackupJob{}
	}
}

// save must be called with s.mu held.
func (s *backupStruct) save() error {
	data, err := json.MarshalIndent(s.jobs, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(backupJobsFilePath(), data, 0o600)
}

func (s *backupStruct) ListJobs() []BackupJob {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]BackupJob{}, s.jobs...)
}

func (s *backupStruct) CreateJob(job BackupJob) (BackupJob, error) {
	s.mu.Lock()
	job.ID = uuid.New().String()
	job.LastRun = ""
	job.LastStatus = ""
	job.LastError = ""
	s.jobs = append(s.jobs, job)
	err := s.save()
	s.mu.Unlock()

	if err != nil {
		return job, err
	}
	s.rescheduleAll()
	return job, nil
}

func (s *backupStruct) UpdateJob(job BackupJob) error {
	s.mu.Lock()
	found := false
	for i, j := range s.jobs {
		if j.ID == job.ID {
			// preserve run history - the edit form doesn't (and shouldn't need to) send it back
			job.LastRun = j.LastRun
			job.LastStatus = j.LastStatus
			job.LastError = j.LastError
			s.jobs[i] = job
			found = true
			break
		}
	}
	if !found {
		s.mu.Unlock()
		return fmt.Errorf("backup job not found")
	}
	err := s.save()
	s.mu.Unlock()

	if err != nil {
		return err
	}
	s.rescheduleAll()
	return nil
}

func (s *backupStruct) DeleteJob(id string) error {
	s.mu.Lock()
	newJobs := make([]BackupJob, 0, len(s.jobs))
	for _, j := range s.jobs {
		if j.ID != id {
			newJobs = append(newJobs, j)
		}
	}
	s.jobs = newJobs
	err := s.save()
	s.mu.Unlock()

	if err != nil {
		return err
	}
	s.rescheduleAll()
	return nil
}

// rescheduleAll tears down and rebuilds every cron entry from the current job list. Simpler and
// less error-prone than trying to diff old vs new schedules, and job counts are small enough
// (personal NAS scale) that this is cheap.
func (s *backupStruct) rescheduleAll() {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, entryID := range s.entryIDs {
		s.cronJob.Remove(entryID)
	}
	s.entryIDs = make(map[string]cron.EntryID)

	for _, job := range s.jobs {
		if !job.Enabled {
			continue
		}

		jobID := job.ID
		entryID, err := s.cronJob.AddFunc(job.Schedule, func() {
			if err := s.RunJob(jobID); err != nil {
				logger.Error("scheduled backup failed", zap.String("job", jobID), zap.Error(err))
			}
		})
		if err != nil {
			logger.Error("failed to schedule backup job", zap.String("job", jobID), zap.String("schedule", job.Schedule), zap.Error(err))
			continue
		}
		s.entryIDs[jobID] = entryID
	}
}

// RunJob runs a backup job synchronously (used by the cron scheduler, and internally by
// RunJobAsync). Blocks for as long as the underlying copy takes - callers on a request/response
// path should use RunJobAsync instead.
func (s *backupStruct) RunJob(id string) error {
	s.mu.Lock()
	var job *BackupJob
	for i := range s.jobs {
		if s.jobs[i].ID == id {
			job = &s.jobs[i]
			break
		}
	}
	if job == nil {
		s.mu.Unlock()
		return fmt.Errorf("backup job not found")
	}
	if s.running[id] {
		s.mu.Unlock()
		return fmt.Errorf("backup job is already running")
	}
	s.running[id] = true
	sourcePath := job.SourcePath
	dstFs := job.RemoteName + ":" + job.RemotePath
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		delete(s.running, id)
		s.mu.Unlock()
	}()

	logger.Info("running backup job", zap.String("id", id), zap.String("source", sourcePath), zap.String("dst", dstFs))
	runErr := httper.SyncCopy(sourcePath, dstFs)

	s.mu.Lock()
	for i := range s.jobs {
		if s.jobs[i].ID == id {
			s.jobs[i].LastRun = time.Now().Format(time.RFC3339)
			if runErr != nil {
				s.jobs[i].LastStatus = "error"
				s.jobs[i].LastError = runErr.Error()
			} else {
				s.jobs[i].LastStatus = "success"
				s.jobs[i].LastError = ""
			}
			break
		}
	}
	_ = s.save()
	s.mu.Unlock()

	return runErr
}

// RunJobAsync starts a backup job in the background and returns immediately, instead of blocking
// for however long the copy takes (up to hours for a large source) - the "run now" HTTP endpoint
// uses this rather than RunJob, since a request/response cycle shouldn't hang open that long.
// Callers should poll ListJobs for LastRun/LastStatus to see when it finishes. Returns an error
// without starting anything if the job doesn't exist or is already running.
func (s *backupStruct) RunJobAsync(id string) error {
	s.mu.Lock()
	found := false
	for i := range s.jobs {
		if s.jobs[i].ID == id {
			found = true
			break
		}
	}
	alreadyRunning := s.running[id]
	s.mu.Unlock()

	if !found {
		return fmt.Errorf("backup job not found")
	}
	if alreadyRunning {
		return fmt.Errorf("backup job is already running")
	}

	go func() {
		if err := s.RunJob(id); err != nil {
			logger.Error("manual backup run failed", zap.String("job", id), zap.Error(err))
		}
	}()
	return nil
}
