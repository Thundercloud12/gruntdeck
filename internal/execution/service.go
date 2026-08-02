package execution

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Thundercloud12/gruntdeck/internal/models"
	"github.com/Thundercloud12/gruntdeck/internal/repository"
	"github.com/Thundercloud12/gruntdeck/internal/ssh"
	"github.com/google/uuid"
)

type StepConfig struct {
	Type       string   `json:"type"`
	Value      string   `json:"value"`
	SourcePath string   `json:"source_path"`
	DestPath   string   `json:"dest_path"`
	JobID      string   `json:"job_id"`
	Args       []string `json:"args"`
}

type Service struct {
	jobs       repository.JobRepository
	inventory  repository.InventoryRepository
	executions repository.ExecutionRepository
	logs       repository.LogRepository
}

func New(
	jobs repository.JobRepository,
	inventory repository.InventoryRepository,
	executions repository.ExecutionRepository,
	logs repository.LogRepository,
) *Service {
	return &Service{
		jobs:       jobs,
		inventory:  inventory,
		executions: executions,
		logs:       logs,
	}
}

func (s *Service) LoadJob(ctx context.Context, executionID string, jobID string) (models.Job, error) {
	job, err := s.jobs.GetJobByID(ctx, jobID)
	if err != nil {
		return models.Job{}, err
	}

	if job == nil {
		return models.Job{}, fmt.Errorf("job %q not found", jobID)
	}
	return *job, nil
}

// ExecuteTarget loads job and target by ID and runs all steps for that target node.
func (s *Service) ExecuteTarget(ctx context.Context, executionID string, jobID string, targetID string) error {
	job, err := s.LoadJob(ctx, executionID, jobID)
	if err != nil {
		return err
	}

	target, err := s.inventory.GetTargetByID(ctx, targetID)
	if err != nil {
		return err
	}
	if target == nil {
		return fmt.Errorf("target %q not found", targetID)
	}

	return s.ExecuteTargetOnNode(ctx, executionID, job, *target)
}

// ExecuteTargetOnNode runs all job steps sequentially on an already loaded target node without redundant DB queries.
func (s *Service) ExecuteTargetOnNode(ctx context.Context, executionID string, job models.Job, target models.Target) error {
	for i, step := range job.Steps {
		if err := s.ExecuteStep(ctx, executionID, target, step); err != nil {
			return fmt.Errorf("step %d failed: %w", i+1, err)
		}
	}
	return nil
}

// ExecuteStep parses step attributes and delegates to the appropriate SSH step handler.
func (s *Service) ExecuteStep(ctx context.Context, executionID string, target models.Target, step models.JobStep) error {
	cfg, err := parseStepConfig(step)
	if err != nil {
		return err
	}

	msg := ""
	switch cfg.Type {
	case "command":
		if cfg.Value == "" {
			return fmt.Errorf("missing command value")
		}
		err := ssh.RunCommand(ctx, target, cfg.Value)
		s.recordLog(ctx, executionID, target.Host, step.ID, "info", fmt.Sprintf("Command: %s", cfg.Value), err)
		return err

	case "script":
		if cfg.SourcePath == "" {
			return fmt.Errorf("missing script source_path")
		}
		msg = fmt.Sprintf("[%s@%s] 📜 Uploading and executing script %s %v...", target.User, target.Host, cfg.SourcePath, cfg.Args)
		fmt.Println(msg)
		err := ssh.RunScript(ctx, target, cfg.SourcePath, cfg.Args)
		s.recordLog(ctx, executionID, target.Host, step.ID, "info", msg, err)
		return err

	case "file-copy":
		if cfg.SourcePath == "" || cfg.DestPath == "" {
			return fmt.Errorf("missing file-copy source_path or dest_path")
		}
		msg = fmt.Sprintf("[%s@%s] 📁 Copying local %s to remote %s...", target.User, target.Host, cfg.SourcePath, cfg.DestPath)
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

	case "job-ref":
		if cfg.JobID == "" {
			return fmt.Errorf("missing job-ref job_id")
		}
		msg = fmt.Sprintf("[%s@%s] 🔗 Invoking job reference: %s", target.User, target.Host, cfg.JobID)
		fmt.Println(msg)
		s.recordLog(ctx, executionID, target.Host, step.ID, "info", msg, nil)
		return s.executeJobRef(ctx, executionID, target, cfg.JobID)

	default:
		return fmt.Errorf("unknown step type: %s", cfg.Type)
	}
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

func parseStepConfig(step models.JobStep) (StepConfig, error) {
	var cfg StepConfig
	cfg.Type = step.Type

	if len(step.Attributes) > 0 {
		if err := json.Unmarshal(step.Attributes, &cfg); err != nil {
			return cfg, fmt.Errorf("failed to parse step attributes: %w", err)
		}
	}
	return cfg, nil
}

func (s *Service) recordLog(ctx context.Context, executionID, targetHost, stepID, level, msg string, err error) {
	if s.logs == nil || executionID == "" {
		return
	}
	if err != nil {
		level = "error"
		msg = fmt.Sprintf("%s | Error: %v", msg, err)
	}
	_ = s.logs.AddLogEntry(ctx, models.LogEntry{
		ID:          uuid.New().String(),
		ExecutionID: executionID,
		TargetID:    targetHost,
		StepID:      stepID,
		Timestamp:   time.Now(),
		Level:       level,
		Message:     msg,
	})
}
