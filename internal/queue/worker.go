package queue

import (
	"context"

	"github.com/Thundercloud12/gruntdeck/internal/execution"
	"github.com/riverqueue/river"
)

type Worker struct {
	river.WorkerDefaults[ExecuteJobArgs]

	service *execution.Service
}

func NewWorker(service *execution.Service) *Worker {
	return &Worker{
		service: service,
	}
}

func (w *Worker) Work(ctx context.Context, job *river.Job[ExecuteJobArgs]) error {
	return w.service.ExecuteTarget(
		ctx,
		job.Args.ExecutionID,
		job.Args.JobID,
		job.Args.TargetID,
	)
}
