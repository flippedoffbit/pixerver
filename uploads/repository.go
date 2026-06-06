package uploads

import (
	"encoding/json"
	"errors"

	"pixerver/store"
)

var ErrNotFound = errors.New("not found")

type Repository interface {
	SaveUpload(*UploadRecord) error
	GetUpload(uploadID string) (*UploadRecord, error)
	SaveJob(*JobRecord) error
	GetJob(jobID string) (*JobRecord, error)
	JobsForUpload(upload *UploadRecord) ([]JobRecord, error)
	Close() error
}

type RedisRepository struct {
	uploads *store.Store
	jobs    *store.Store
}

func NewRedisRepository() (*RedisRepository, error) {
	uploadStore, err := store.New("uploads:")
	if err != nil {
		return nil, err
	}
	jobStore, err := store.New("jobs:")
	if err != nil {
		_ = uploadStore.Close()
		return nil, err
	}
	return &RedisRepository{uploads: uploadStore, jobs: jobStore}, nil
}

func (r *RedisRepository) Close() error {
	if r == nil {
		return nil
	}
	if r.uploads != nil {
		_ = r.uploads.Close()
	}
	if r.jobs != nil {
		_ = r.jobs.Close()
	}
	return nil
}

func (r *RedisRepository) SaveUpload(upload *UploadRecord) error {
	b, err := json.Marshal(upload)
	if err != nil {
		return err
	}
	return r.uploads.SetString(upload.UploadID, b)
}

func (r *RedisRepository) GetUpload(uploadID string) (*UploadRecord, error) {
	b, err := r.uploads.GetString(uploadID)
	if err != nil {
		return nil, ErrNotFound
	}
	var upload UploadRecord
	if err := json.Unmarshal(b, &upload); err != nil {
		return nil, err
	}
	return &upload, nil
}

func (r *RedisRepository) SaveJob(job *JobRecord) error {
	b, err := json.Marshal(job)
	if err != nil {
		return err
	}
	return r.jobs.SetString(job.JobID, b)
}

func (r *RedisRepository) GetJob(jobID string) (*JobRecord, error) {
	b, err := r.jobs.GetString(jobID)
	if err != nil {
		return nil, ErrNotFound
	}
	var job JobRecord
	if err := json.Unmarshal(b, &job); err != nil {
		return nil, err
	}
	return &job, nil
}

func (r *RedisRepository) JobsForUpload(upload *UploadRecord) ([]JobRecord, error) {
	jobs := make([]JobRecord, 0, len(upload.JobIDs))
	for _, jobID := range upload.JobIDs {
		job, err := r.GetJob(jobID)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, *job)
	}
	return jobs, nil
}
