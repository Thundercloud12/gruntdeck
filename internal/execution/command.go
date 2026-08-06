package execution

import (
	"context"
	"fmt"

	"github.com/Thundercloud12/gruntdeck/internal/models"
	"github.com/Thundercloud12/gruntdeck/internal/ssh"
	"github.com/Thundercloud12/gruntdeck/internal/variables"
)

func (s *Service) handleCommandStep(ctx context.Context, execCtx models.ExecutionContext, step models.JobStep, cfg StepConfig) error {
	if cfg.Value == "" {
		return fmt.Errorf("missing command value")
	}

	cmdValue := variables.Resolve(cfg.Value, execCtx)

	s.recordLog(ctx, execCtx.Execution.ID, execCtx.Target.Host, step.ID, "info", fmt.Sprintf("Command: %s", cmdValue), nil)

	cred, err := s.ResolveCredential(ctx, execCtx.Target)
	if err != nil {
		s.recordLog(ctx, execCtx.Execution.ID, execCtx.Target.Host, step.ID, "error", fmt.Sprintf("Credential resolution failed: %v", err), err)
		return err
	}

	err = ssh.RunCommandWithCredential(ctx, execCtx.Target, cmdValue, cred, func(line string, isErr bool) {
		level := "info"
		if isErr {
			level = "error"
		}
		s.recordLog(ctx, execCtx.Execution.ID, execCtx.Target.Host, step.ID, level, line, nil)
	})

	if err != nil {
		s.recordLog(ctx, execCtx.Execution.ID, execCtx.Target.Host, step.ID, "error", fmt.Sprintf("Command failed: %v", err), err)
	}

	return err
}
