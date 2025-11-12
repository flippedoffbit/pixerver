package models

import (
	"pixerver/internal/uuidv7"
)

/*
Job represents a job in the system.
its split of conversion jobs from input tokens. so basically a job single unit of work
that the system processes. and jobs are just broken up combintions of resolutions, transformers,
and types (we have not broken up destination backends as its pointless to rencode images just for writing them to different storage backends).
*/
type Job struct {
	ID                    string            `json:"id"`
	SourceFileName        string            `json:"sourceFileName"`
	Type                  string            `json:"type"`
	Status                string            `json:"status"`
	Settings              map[string]string `json:"settings"`
	TransformerID         string            `json:"transformerId"`
	Resolution            Resolution        `json:"resolution"`
	DestinationBackendIDs []string          `json:"destinationBackendIds"`
}

// ConversionJobs is a convenience alias for a slice of ConversionJob
// defined in models/inputToken.go. The methods here convert those
// conversion job descriptors into concrete Job instances the system
// processes.
type ConversionJobs []ConversionJob

// ToJobList converts a slice of ConversionJob into a flat slice of Job.
// Each ConversionJob may expand into multiple Jobs (one per resolution).
// The resMap parameter is used to look up resolution names; unknown
// resolutions are skipped. The returned slice is a plain list — callers
// can rely on Job.ID for uniqueness.
// ToJobs converts a slice of ConversionJob into a flat slice of Job. This
// keeps the API consistent with ConversionJob.ToJobs and provides a simple
// flat list of jobs where each job has its own ID.
func (cjs ConversionJobs) ToJobs(resMap map[string]Resolution) []Job {
	var out []Job
	for _, cj := range cjs {
		for _, rname := range cj.Resolutions {
			res, ok := resMap[rname]
			if !ok {
				continue
			}

			job := Job{
				ID:                    uuidv7.New(),
				Type:                  cj.Type,
				Status:                "pending",
				Settings:              cj.Settings,
				TransformerID:         "",
				Resolution:            res,
				DestinationBackendIDs: cj.DestinationBackends,
			}

			if len(cj.Transformers) > 0 {
				job.TransformerID = cj.Transformers[0]
			}

			out = append(out, job)
		}
	}
	return out
}
