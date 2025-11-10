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

// ToJobMap converts a slice of ConversionJob into a map keyed by Job.ID.
// Using the job ID as the key ensures that jobs with the same Type
// (including cloned/duplicate types) are preserved individually and
// not collapsed together. The resMap parameter is expected to contain
// the named resolutions referenced by each ConversionJob (e.g. "large",
// "thumbnail"). Jobs referencing an unknown resolution are skipped.
func (cjs ConversionJobs) ToJobMap(resMap map[string]Resolution) map[string]Job {
	out := make(map[string]Job)
	for _, cj := range cjs {
		jobs := cj.ToJobs(resMap)
		if len(jobs) == 0 {
			continue
		}
		for _, j := range jobs {
			out[j.ID] = j
		}
	}
	return out
}

// ToJobs expands a single ConversionJob into one Job per resolution
// (using the provided resMap to resolve resolution names). Unknown
// resolutions are ignored. The returned Jobs have Status set to
// "pending" and Settings are preserved as map[string]any; callers may
// stringify values when passing to encoders.
func (cj ConversionJob) ToJobs(resMap map[string]Resolution) []Job {
	var out []Job
	for _, rname := range cj.Resolutions {
		res, ok := resMap[rname]
		if !ok {
			// skip unknown resolution names
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
	return out
}

// (Settings remain map[string]string; no helpers required.)
