package execution

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
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

// ExecuteJob orchestrates parallel execution across all matching targets for a job.
func (s *Service) ExecuteJob(ctx context.Context, executionID string, jobID string) error {
	job, err := s.LoadJob(ctx, executionID, jobID)
	if err != nil {
		return err
	}

	matchedTargets, err := s.inventory.GetTargetByTags(ctx, job.TargetFilter)
	if err != nil {
		return fmt.Errorf("error loading targets from database: %w", err)
	}

	if len(matchedTargets) == 0 {
		return fmt.Errorf("no targets matched the criteria for job %q (filter: %v)", job.Name, job.TargetFilter)
	}

	if s.executions != nil {
		err := s.executions.CreateExecution(ctx, models.Execution{
			ID:           executionID,
			JobID:        jobID,
			Status:       "running",
			StartedAt:    time.Now(),
			TargetsTotal: len(matchedTargets),
		})
		if err != nil {
			fmt.Printf("Warning: Failed to create execution record in database: %v\n", err)
		}
	}

	fmt.Printf("Job: %s | Matching Nodes: %d\n", job.Name, len(matchedTargets))
	fmt.Println(strings.Repeat("=", 60))

	type targetResult struct {
		target models.Target
		err    error
	}

	results := make(chan targetResult, len(matchedTargets))
	var wg sync.WaitGroup

	for _, t := range matchedTargets {
		wg.Add(1)
		go func(target models.Target) {
			defer wg.Done()
			err := s.ExecuteTarget(ctx, executionID, jobID, target.ID)
			results <- targetResult{target: target, err: err}
		}(t)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	var failed, succeeded int
	for res := range results {
		if res.err != nil {
			fmt.Printf("❌ [SYSTEM] %s@%s failed: %v\n", res.target.User, res.target.Host, res.err)
			failed++
		} else {
			succeeded++
		}
	}

	fmt.Println(strings.Repeat("=", 60))
	fmt.Printf("Execution Summary: %d Succeeded | %d Failed\n", succeeded, failed)

	if s.executions != nil && executionID != "" {
		status := "succeeded"
		if failed > 0 {
			status = "failed"
		}
		_ = s.executions.UpdateExecution(ctx, models.Execution{
			ID:               executionID,
			JobID:            jobID,
			Status:           status,
			EndedAt:          time.Now(),
			TargetsTotal:     len(matchedTargets),
			TargetsSucceeded: succeeded,
			TargetsFailed:    failed,
		})
	}

	if failed > 0 {
		return fmt.Errorf("execution completed with %d failed target(s)", failed)
	}
	return nil
}

// ExecuteTarget runs all job steps sequentially for a single target node.
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

	for i, step := range job.Steps {
		if err := s.ExecuteStep(ctx, executionID, *target, step); err != nil {
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
	refJob, err := s.jobs.GetJobByID(ctx, refJobID)
	if err != nil {
		return fmt.Errorf("job reference %q not found in database: %w", refJobID, err)
	}
	if refJob == nil {
		return fmt.Errorf("job reference %q not found", refJobID)
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
