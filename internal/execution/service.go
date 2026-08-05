package execution

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

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

// ExecuteTarget loads job and target by ID, builds ExecutionContext, updates status, and executes steps.
func (s *Service) ExecuteTarget(ctx context.Context, executionID string, jobID string, targetID string) error {
	var execRecord *models.Execution
	if s.executions != nil {
		if rec, err := s.executions.GetExecutionByID(ctx, executionID); err == nil && rec != nil {
			execRecord = rec
			execRecord.Status = "running"
			_ = s.executions.UpdateExecution(ctx, *execRecord)
		}
	}

	job, err := s.LoadJob(ctx, executionID, jobID)
	if err != nil {
		s.markExecutionFailed(ctx, executionID)
		return err
	}

	target, err := s.inventory.GetTargetByID(ctx, targetID)
	if err != nil {
		s.markExecutionFailed(ctx, executionID)
		return err
	}
	if target == nil {
		s.markExecutionFailed(ctx, executionID)
		return fmt.Errorf("target %q not found", targetID)
	}

	execCtx := models.ExecutionContext{
		Job:    job,
		Target: *target,
	}

	if execRecord != nil {
		execCtx.Execution = *execRecord
		execCtx.Options = execRecord.Options
	}

	if err := s.ExecuteTargetOnNode(ctx, execCtx); err != nil {
		s.markExecutionFailed(ctx, executionID)
		return err
	}

	// Transition status to "succeeded"
	if s.executions != nil && execRecord != nil {
		execRecord.Status = "succeeded"
		execRecord.TargetsSucceeded += 1
		execRecord.EndedAt = time.Now()
		_ = s.executions.UpdateExecution(ctx, *execRecord)
	}

	return nil
}

func (s *Service) markExecutionFailed(ctx context.Context, executionID string) {
	if s.executions == nil {
		return
	}
	if execRecord, err := s.executions.GetExecutionByID(ctx, executionID); err == nil && execRecord != nil {
		execRecord.Status = "failed"
		execRecord.TargetsFailed += 1
		execRecord.EndedAt = time.Now()
		_ = s.executions.UpdateExecution(ctx, *execRecord)
	}
}

// ExecuteTargetOnNode runs all job steps sequentially on a target node using ExecutionContext.
func (s *Service) ExecuteTargetOnNode(ctx context.Context, execCtx models.ExecutionContext) error {
	for i, step := range execCtx.Job.Steps {
		if err := s.ExecuteStep(ctx, execCtx, step); err != nil {
			return fmt.Errorf("step %d failed: %w", i+1, err)
		}
	}
	return nil
}

// ExecuteStep parses step attributes and delegates to step handlers using ExecutionContext.
func (s *Service) ExecuteStep(ctx context.Context, execCtx models.ExecutionContext, step models.JobStep) error {
	cfg, err := parseStepConfig(step)
	if err != nil {
		return err
	}

	switch cfg.Type {
	case "command":
		return s.handleCommandStep(ctx, execCtx, step, cfg)

	case "script":
		return s.handleScriptStep(ctx, execCtx, step, cfg)

	case "file-copy":
		return s.handleFileCopyStep(ctx, execCtx, step, cfg)

	case "job-ref":
		return s.handleJobRefStep(ctx, execCtx, step, cfg)

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
