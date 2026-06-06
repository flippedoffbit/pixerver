package uploads

import (
	"time"

	"pixerver/models"
	"pixerver/pipeline"
)

const (
	UploadStatusQueued        = "queued"
	UploadStatusProcessing    = "processing"
	UploadStatusCompleted     = "completed"
	UploadStatusPartialFailed = "partial_failed"
	UploadStatusFailed        = "failed"

	JobStatusQueued     = "queued"
	JobStatusProcessing = "processing"
	JobStatusCompleted  = "completed"
	JobStatusFailed     = "failed"
)

// UploadRecord is the durable state for one uploaded source file.
type UploadRecord struct {
	UploadID       string             `json:"uploadId"`
	SourceFileName string             `json:"sourceFileName"`
	SourcePath     string             `json:"sourcePath"`
	Status         string             `json:"status"`
	Token          *models.InputToken `json:"token"`
	JobIDs         []string           `json:"jobIds"`
	CallbackSent   bool               `json:"callbackSent"`
	CallbackStatus string             `json:"callbackStatus,omitempty"`
	CallbackError  string             `json:"callbackError,omitempty"`
	CallbackAt     *time.Time         `json:"callbackAt,omitempty"`
	CreatedAt      time.Time          `json:"createdAt"`
	UpdatedAt      time.Time          `json:"updatedAt"`
	CompletedAt    *time.Time         `json:"completedAt,omitempty"`
	LastError      string             `json:"lastError,omitempty"`
}

// JobRecord is the durable state for one concrete image conversion job.
type JobRecord struct {
	JobID                 string              `json:"jobId"`
	UploadID              string              `json:"uploadId"`
	SourceFileName        string              `json:"sourceFileName"`
	SourcePath            string              `json:"sourcePath"`
	Type                  string              `json:"type"`
	Status                string              `json:"status"`
	Settings              map[string]string   `json:"settings,omitempty"`
	TransformerID         string              `json:"transformerId,omitempty"`
	Resolution            models.Resolution   `json:"resolution"`
	DestinationBackendIDs []string            `json:"destinationBackendIds"`
	Attempts              int                 `json:"attempts"`
	Artifacts             []pipeline.Artifact `json:"artifacts,omitempty"`
	CreatedAt             time.Time           `json:"createdAt"`
	UpdatedAt             time.Time           `json:"updatedAt"`
	StartedAt             *time.Time          `json:"startedAt,omitempty"`
	CompletedAt           *time.Time          `json:"completedAt,omitempty"`
	LastError             string              `json:"lastError,omitempty"`
}

type UploadResponse struct {
	UploadID string       `json:"uploadId"`
	Filename string       `json:"filename"`
	Path     string       `json:"path"`
	Status   string       `json:"status"`
	Jobs     []JobSummary `json:"jobs"`
}

type JobSummary struct {
	JobID                 string            `json:"jobId"`
	UploadID              string            `json:"uploadId"`
	Type                  string            `json:"type"`
	Status                string            `json:"status"`
	Resolution            models.Resolution `json:"resolution"`
	DestinationBackendIDs []string          `json:"destinationBackendIds"`
}

type UploadStatusResponse struct {
	UploadID       string              `json:"uploadId"`
	Status         string              `json:"status"`
	Filename       string              `json:"filename"`
	Path           string              `json:"path"`
	SourceFileName string              `json:"sourceFileName"`
	Jobs           []JobRecord         `json:"jobs"`
	Artifacts      []pipeline.Artifact `json:"artifacts"`
	LastError      string              `json:"lastError,omitempty"`
	CallbackSent   bool                `json:"callbackSent"`
	CallbackStatus string              `json:"callbackStatus,omitempty"`
	CallbackError  string              `json:"callbackError,omitempty"`
	CreatedAt      time.Time           `json:"createdAt"`
	UpdatedAt      time.Time           `json:"updatedAt"`
	CompletedAt    *time.Time          `json:"completedAt,omitempty"`
}

func summaryFromJob(job JobRecord) JobSummary {
	return JobSummary{
		JobID:                 job.JobID,
		UploadID:              job.UploadID,
		Type:                  job.Type,
		Status:                job.Status,
		Resolution:            job.Resolution,
		DestinationBackendIDs: append([]string(nil), job.DestinationBackendIDs...),
	}
}
