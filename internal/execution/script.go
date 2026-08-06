package execution

import (
	"context"
	"fmt"

	"github.com/Thundercloud12/gruntdeck/internal/models"
	"github.com/Thundercloud12/gruntdeck/internal/ssh"
	"github.com/Thundercloud12/gruntdeck/internal/variables"
)

func (s *Service) handleScriptStep(ctx context.Context, execCtx models.ExecutionContext, step models.JobStep, cfg StepConfig) error {
	if cfg.SourcePath == "" {
		return fmt.Errorf("missing script source_path")
	}

	sourcePath := variables.Resolve(cfg.SourcePath, execCtx)
	resolvedArgs := make([]string, len(cfg.Args))
	for i, arg := range cfg.Args {
		resolvedArgs[i] = variables.Resolve(arg, execCtx)
	}

	msg := fmt.Sprintf("[%s@%s] 📜 Uploading and executing script %s %v...", execCtx.Target.User, execCtx.Target.Host, sourcePath, resolvedArgs)
	s.recordLog(ctx, execCtx.Execution.ID, execCtx.Target.Host, step.ID, "info", msg, nil)

	cred, err := s.ResolveCredential(ctx, execCtx.Target)
	if err != nil {
		s.recordLog(ctx, execCtx.Execution.ID, execCtx.Target.Host, step.ID, "error", fmt.Sprintf("Credential resolution failed: %v", err), err)
		return err
	}

	err = ssh.RunScriptWithCredential(ctx, execCtx.Target, sourcePath, resolvedArgs, cred, func(line string, isErr bool) {
		level := "info"
		if isErr {
			level = "error"
		}
		s.recordLog(ctx, execCtx.Execution.ID, execCtx.Target.Host, step.ID, level, line, nil)
	})

	if err != nil {
		s.recordLog(ctx, execCtx.Execution.ID, execCtx.Target.Host, step.ID, "error", fmt.Sprintf("Script failed: %v", err), err)
	}

	return err
}
