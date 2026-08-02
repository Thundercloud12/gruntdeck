package execution

import (
	"context"
	"fmt"

	"github.com/Thundercloud12/gruntdeck/internal/models"
)

func (s *Service) handleJobRefStep(ctx context.Context, executionID string, target models.Target, step models.JobStep, cfg StepConfig) error {
	if cfg.JobID == "" {
		return fmt.Errorf("missing job-ref job_id")
	}
	msg := fmt.Sprintf("[%s@%s] 🔗 Invoking job reference: %s", target.User, target.Host, cfg.JobID)
	fmt.Println(msg)
	s.recordLog(ctx, executionID, target.Host, step.ID, "info", msg, nil)
	return s.executeJobRef(ctx, executionID, target, cfg.JobID)
}

func (s *Service) executeJobRef(ctx context.Context, executionID string, target models.Target, refJobID string) error {
	refJob, err := s.LoadJob(ctx, executionID, refJobID)
	if err != nil {
		return fmt.Errorf("job reference %q not found in database: %w", refJobID, err)
	}

	for i, step := range refJob.Steps {
		if err := s.ExecuteStep(ctx, executionID, target, step); err != nil {
			return fmt.Errorf("under job-ref %q step %d: %w", refJobID, i+1, err)
		}
	}
	return nil
}
