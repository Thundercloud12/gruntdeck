package execution

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Thundercloud12/gruntdeck/internal/models"
	"github.com/Thundercloud12/gruntdeck/internal/repository"
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

// ExecuteTargetOnNode runs all job steps sequentially on an loaded target node.
func (s *Service) ExecuteTargetOnNode(ctx context.Context, executionID string, job models.Job, target models.Target) error {
	for i, step := range job.Steps {
		if err := s.ExecuteStep(ctx, executionID, target, step); err != nil {
			return fmt.Errorf("step %d failed: %w", i+1, err)
		}
	}
	return nil
}

// ExecuteStep parses step attributes and delegates to step handlers.
func (s *Service) ExecuteStep(ctx context.Context, executionID string, target models.Target, step models.JobStep) error {
	cfg, err := parseStepConfig(step)
	if err != nil {
		return err
	}

	switch cfg.Type {
	case "command":
		return s.handleCommandStep(ctx, executionID, target, step, cfg)

	case "script":
		return s.handleScriptStep(ctx, executionID, target, step, cfg)

	case "file-copy":
		return s.handleFileCopyStep(ctx, executionID, target, step, cfg)

	case "job-ref":
		return s.handleJobRefStep(ctx, executionID, target, step, cfg)

	default:
		return fmt.Errorf("unknown step type: %s", cfg.Type)
	}
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
