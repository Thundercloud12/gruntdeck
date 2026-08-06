package execution

import (
	"context"
	"fmt"

	"github.com/Thundercloud12/gruntdeck/internal/models"
	"github.com/Thundercloud12/gruntdeck/internal/ssh"
	"github.com/Thundercloud12/gruntdeck/internal/variables"
)

func (s *Service) handleFileCopyStep(ctx context.Context, execCtx models.ExecutionContext, step models.JobStep, cfg StepConfig) error {
	if cfg.SourcePath == "" || cfg.DestPath == "" {
		return fmt.Errorf("missing file-copy source_path or dest_path")
	}

	sourcePath := variables.Resolve(cfg.SourcePath, execCtx)
	destPath := variables.Resolve(cfg.DestPath, execCtx)

	msg := fmt.Sprintf("[%s@%s] 📁 Copying local %s to remote %s...", execCtx.Target.User, execCtx.Target.Host, sourcePath, destPath)
	fmt.Println(msg)

	cred, err := s.ResolveCredential(ctx, execCtx.Target)
	if err != nil {
		s.recordLog(ctx, execCtx.Execution.ID, execCtx.Target.Host, step.ID, "error", fmt.Sprintf("Credential resolution failed: %v", err), err)
		return err
	}

	err = ssh.CopyFileWithCredential(ctx, execCtx.Target, sourcePath, destPath, cred)
	if err == nil {
		successMsg := fmt.Sprintf("[%s@%s] 📁 Successfully copied %s", execCtx.Target.User, execCtx.Target.Host, destPath)
		fmt.Println(successMsg)
		s.recordLog(ctx, execCtx.Execution.ID, execCtx.Target.Host, step.ID, "info", successMsg, nil)
	} else {
		s.recordLog(ctx, execCtx.Execution.ID, execCtx.Target.Host, step.ID, "error", msg, err)
	}
	return err
}
