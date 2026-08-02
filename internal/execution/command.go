package execution

import (
	"context"
	"fmt"

	"github.com/Thundercloud12/gruntdeck/internal/models"
	"github.com/Thundercloud12/gruntdeck/internal/ssh"
)

func (s *Service) handleCommandStep(ctx context.Context, executionID string, target models.Target, step models.JobStep, cfg StepConfig) error {
	if cfg.Value == "" {
		return fmt.Errorf("missing command value")
	}
	err := ssh.RunCommand(ctx, target, cfg.Value)
	s.recordLog(ctx, executionID, target.Host, step.ID, "info", fmt.Sprintf("Command: %s", cfg.Value), err)
	return err
}
