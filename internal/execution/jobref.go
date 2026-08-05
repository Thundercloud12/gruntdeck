package execution

import (
	"context"
	"fmt"

	"github.com/Thundercloud12/gruntdeck/internal/models"
	"github.com/Thundercloud12/gruntdeck/internal/variables"
)

func (s *Service) handleJobRefStep(ctx context.Context, execCtx models.ExecutionContext, step models.JobStep, cfg StepConfig) error {
	if cfg.JobID == "" {
		return fmt.Errorf("missing job-ref job_id")
	}

	refJobID := variables.Resolve(cfg.JobID, execCtx)

	msg := fmt.Sprintf("[%s@%s] 🔗 Invoking job reference: %s", execCtx.Target.User, execCtx.Target.Host, refJobID)
	fmt.Println(msg)
	s.recordLog(ctx, execCtx.Execution.ID, execCtx.Target.Host, step.ID, "info", msg, nil)
	return s.executeJobRef(ctx, execCtx, refJobID)
}

func (s *Service) executeJobRef(ctx context.Context, execCtx models.ExecutionContext, refJobID string) error {
	refJob, err := s.LoadJob(ctx, execCtx.Execution.ID, refJobID)
	if err != nil {
		return fmt.Errorf("job reference %q not found in database: %w", refJobID, err)
	}

	refExecCtx := execCtx
	refExecCtx.Job = refJob

	for i, step := range refJob.Steps {
		if err := s.ExecuteStep(ctx, refExecCtx, step); err != nil {
			return fmt.Errorf("under job-ref %q step %d: %w", refJobID, i+1, err)
		}
	}
	return nil
}
