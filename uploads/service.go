package uploads

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"path/filepath"
	"time"

	"pixerver/internal/uuidv7"
	"pixerver/models"
	"pixerver/pipeline"
)

type Producer interface {
	Produce(values map[string]interface{}) (string, error)
}

type TokenSigner interface {
	SignUpload(context.Context, string) (string, error)
}

type Service struct {
	Repo        Repository
	Queue       Producer
	Processor   pipeline.Processor
	MaxAttempts int
	Signer      TokenSigner
	Now         func() time.Time
}

func (s *Service) CreateUpload(ctx context.Context, sourceName, sourcePath string, token *models.InputToken) (UploadResponse, error) {
	if s == nil || s.Repo == nil || s.Queue == nil {
		return UploadResponse{}, errors.New("upload queue unavailable")
	}
	if err := token.Validate(); err != nil {
		return UploadResponse{}, err
	}
	now := s.now()
	uploadID := uuidv7.New()
	concreteJobs := models.ConversionJobs(token.ConversionJobs).ToJobs(token.Resolutions, sourceName)

	upload := &UploadRecord{
		UploadID:       uploadID,
		SourceFileName: sourceName,
		SourcePath:     sourcePath,
		Status:         UploadStatusQueued,
		Token:          token,
		JobIDs:         make([]string, 0, len(concreteJobs)),
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	jobRecords := make([]JobRecord, 0, len(concreteJobs))
	for _, job := range concreteJobs {
		record := JobRecord{
			JobID:                 job.ID,
			UploadID:              uploadID,
			SourceFileName:        sourceName,
			SourcePath:            sourcePath,
			Type:                  job.Type,
			Status:                JobStatusQueued,
			Settings:              maps.Clone(job.Settings),
			TransformerID:         job.TransformerID,
			Resolution:            job.Resolution,
			DestinationBackendIDs: append([]string(nil), job.DestinationBackendIDs...),
			CreatedAt:             now,
			UpdatedAt:             now,
		}
		upload.JobIDs = append(upload.JobIDs, record.JobID)
		jobRecords = append(jobRecords, record)
	}

	if err := s.Repo.SaveUpload(upload); err != nil {
		return UploadResponse{}, err
	}
	for i := range jobRecords {
		if err := s.Repo.SaveJob(&jobRecords[i]); err != nil {
			return UploadResponse{}, err
		}
	}
	for _, job := range jobRecords {
		if _, err := s.Queue.Produce(map[string]interface{}{"uploadId": uploadID, "jobId": job.JobID}); err != nil {
			return UploadResponse{}, err
		}
	}

	summaries := make([]JobSummary, 0, len(jobRecords))
	for _, job := range jobRecords {
		summaries = append(summaries, summaryFromJob(job))
	}
	return UploadResponse{
		UploadID: uploadID,
		Filename: sourceName,
		Path:     sourcePath,
		Status:   upload.Status,
		Jobs:     summaries,
	}, nil
}

func (s *Service) GetUploadStatus(ctx context.Context, uploadID string) (UploadStatusResponse, error) {
	if s == nil || s.Repo == nil {
		return UploadStatusResponse{}, errors.New("upload store unavailable")
	}
	upload, err := s.Repo.GetUpload(uploadID)
	if err != nil {
		return UploadStatusResponse{}, err
	}
	jobs, err := s.Repo.JobsForUpload(upload)
	if err != nil {
		return UploadStatusResponse{}, err
	}
	return buildStatusResponse(upload, jobs), nil
}

func (s *Service) ProcessJob(ctx context.Context, uploadID, jobID string) error {
	if s == nil || s.Repo == nil {
		return errors.New("upload store unavailable")
	}
	upload, err := s.Repo.GetUpload(uploadID)
	if err != nil {
		return err
	}
	job, err := s.Repo.GetJob(jobID)
	if err != nil {
		return err
	}
	if job.Status == JobStatusCompleted || job.Status == JobStatusFailed {
		return nil
	}

	now := s.now()
	job.Status = JobStatusProcessing
	job.Attempts++
	job.StartedAt = &now
	job.UpdatedAt = now
	job.LastError = ""
	upload.Status = UploadStatusProcessing
	upload.UpdatedAt = now
	if err := s.Repo.SaveUpload(upload); err != nil {
		return err
	}
	if err := s.Repo.SaveJob(job); err != nil {
		return err
	}

	modelJob := models.Job{
		ID:                    job.JobID,
		SourceFileName:        job.SourceFileName,
		Type:                  job.Type,
		Status:                job.Status,
		Settings:              maps.Clone(job.Settings),
		TransformerID:         job.TransformerID,
		Resolution:            job.Resolution,
		DestinationBackendIDs: append([]string(nil), job.DestinationBackendIDs...),
	}
	artifacts, processErr := s.Processor.ProcessJob(ctx, upload.Token, job.SourcePath, modelJob)

	now = s.now()
	job.UpdatedAt = now
	if processErr != nil {
		job.LastError = processErr.Error()
		if job.Attempts >= s.maxAttempts() {
			job.Status = JobStatusFailed
			job.CompletedAt = &now
		} else {
			job.Status = JobStatusQueued
		}
		if err := s.Repo.SaveJob(job); err != nil {
			return err
		}
		if err := s.recomputeUpload(ctx, upload.UploadID); err != nil {
			return err
		}
		return processErr
	}

	job.Status = JobStatusCompleted
	job.Artifacts = artifacts
	job.CompletedAt = &now
	if err := s.Repo.SaveJob(job); err != nil {
		return err
	}
	return s.recomputeUpload(ctx, upload.UploadID)
}

func (s *Service) recomputeUpload(ctx context.Context, uploadID string) error {
	upload, err := s.Repo.GetUpload(uploadID)
	if err != nil {
		return err
	}
	jobs, err := s.Repo.JobsForUpload(upload)
	if err != nil {
		return err
	}
	status := aggregateStatus(jobs)
	now := s.now()
	upload.Status = status
	upload.UpdatedAt = now
	upload.LastError = firstJobError(jobs)
	if isTerminalUploadStatus(status) && upload.CompletedAt == nil {
		upload.CompletedAt = &now
	}
	if err := s.Repo.SaveUpload(upload); err != nil {
		return err
	}
	if isTerminalUploadStatus(status) && !upload.CallbackSent {
		return s.sendTerminalCallback(ctx, upload, jobs)
	}
	return nil
}

func (s *Service) sendTerminalCallback(ctx context.Context, upload *UploadRecord, jobs []JobRecord) error {
	now := s.now()
	upload.CallbackSent = true
	upload.CallbackAt = &now
	if s.Signer == nil {
		upload.CallbackStatus = "sign_failed"
		upload.CallbackError = "callback jwt signer is not configured"
		upload.UpdatedAt = now
		return s.Repo.SaveUpload(upload)
	}
	bearer, err := s.Signer.SignUpload(ctx, upload.UploadID)
	if err != nil {
		upload.CallbackStatus = "sign_failed"
		upload.CallbackError = err.Error()
		upload.UpdatedAt = now
		return s.Repo.SaveUpload(upload)
	}

	payload := buildStatusResponse(upload, jobs)
	if err := pipeline.SendCallback(ctx, s.Processor.HTTPClient, upload.Token.CallbackURL, payload, bearer); err != nil {
		upload.CallbackStatus = "failed"
		upload.CallbackError = err.Error()
	} else {
		upload.CallbackStatus = "sent"
		upload.CallbackError = ""
	}
	upload.UpdatedAt = now
	return s.Repo.SaveUpload(upload)
}

func (s *Service) now() time.Time {
	if s != nil && s.Now != nil {
		return s.Now()
	}
	return time.Now().UTC()
}

func (s *Service) maxAttempts() int {
	if s != nil && s.MaxAttempts > 0 {
		return s.MaxAttempts
	}
	return 3
}

func buildStatusResponse(upload *UploadRecord, jobs []JobRecord) UploadStatusResponse {
	artifacts := make([]pipeline.Artifact, 0)
	for _, job := range jobs {
		artifacts = append(artifacts, job.Artifacts...)
	}
	return UploadStatusResponse{
		UploadID:       upload.UploadID,
		Status:         upload.Status,
		Filename:       filepath.Base(upload.SourcePath),
		Path:           upload.SourcePath,
		SourceFileName: upload.SourceFileName,
		Jobs:           jobs,
		Artifacts:      artifacts,
		LastError:      upload.LastError,
		CallbackSent:   upload.CallbackSent,
		CallbackStatus: upload.CallbackStatus,
		CallbackError:  upload.CallbackError,
		CreatedAt:      upload.CreatedAt,
		UpdatedAt:      upload.UpdatedAt,
		CompletedAt:    upload.CompletedAt,
	}
}

func aggregateStatus(jobs []JobRecord) string {
	if len(jobs) == 0 {
		return UploadStatusFailed
	}
	completed := 0
	failed := 0
	processing := 0
	for _, job := range jobs {
		switch job.Status {
		case JobStatusCompleted:
			completed++
		case JobStatusFailed:
			failed++
		case JobStatusProcessing:
			processing++
		}
	}
	if completed == len(jobs) {
		return UploadStatusCompleted
	}
	if failed == len(jobs) {
		return UploadStatusFailed
	}
	if completed+failed == len(jobs) {
		return UploadStatusPartialFailed
	}
	if processing > 0 || completed > 0 || failed > 0 {
		return UploadStatusProcessing
	}
	return UploadStatusQueued
}

func firstJobError(jobs []JobRecord) string {
	for _, job := range jobs {
		if job.LastError != "" {
			return fmt.Sprintf("job %s: %s", job.JobID, job.LastError)
		}
	}
	return ""
}

func isTerminalUploadStatus(status string) bool {
	return status == UploadStatusCompleted || status == UploadStatusPartialFailed || status == UploadStatusFailed
}
