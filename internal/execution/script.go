package execution

import (
	"context"
	"fmt"

	"github.com/Thundercloud12/gruntdeck/internal/models"
	"github.com/Thundercloud12/gruntdeck/internal/ssh"
)

func (s *Service) handleScriptStep(ctx context.Context, executionID string, target models.Target, step models.JobStep, cfg StepConfig) error {
	if cfg.SourcePath == "" {
		return fmt.Errorf("missing script source_path")
	}
	msg := fmt.Sprintf("[%s@%s] 📜 Uploading and executing script %s %v...", target.User, target.Host, cfg.SourcePath, cfg.Args)
	fmt.Println(msg)
	err := ssh.RunScript(ctx, target, cfg.SourcePath, cfg.Args)
	s.recordLog(ctx, executionID, target.Host, step.ID, "info", msg, err)
	return err
}
