package scheduler

import "github.com/Thundercloud12/gruntdeck/internal/job"

// Scheduler handles periodic triggering of jobs based on cron schedules.
type Scheduler interface {
	Start() error
	Stop() error
	AddJob(j *job.Job) error
	RemoveJob(jobID string) error
	IsScheduled(jobID string) bool
}
