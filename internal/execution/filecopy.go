package execution

import (
	"context"
	"fmt"

	"github.com/Thundercloud12/gruntdeck/internal/models"
	"github.com/Thundercloud12/gruntdeck/internal/ssh"
)

func (s *Service) handleFileCopyStep(ctx context.Context, executionID string, target models.Target, step models.JobStep, cfg StepConfig) error {
	if cfg.SourcePath == "" || cfg.DestPath == "" {
		return fmt.Errorf("missing file-copy source_path or dest_path")
	}
	msg := fmt.Sprintf("[%s@%s] 📁 Copying local %s to remote %s...", target.User, target.Host, cfg.SourcePath, cfg.DestPath)
	fmt.Println(msg)
	err := ssh.CopyFile(ctx, target, cfg.SourcePath, cfg.DestPath)
	if err == nil {
		successMsg := fmt.Sprintf("[%s@%s] 📁 Successfully copied %s", target.User, target.Host, cfg.DestPath)
		fmt.Println(successMsg)
		s.recordLog(ctx, executionID, target.Host, step.ID, "info", successMsg, nil)
	} else {
		s.recordLog(ctx, executionID, target.Host, step.ID, "error", msg, err)
	}
	return err
}
