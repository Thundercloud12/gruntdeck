package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Thundercloud12/gruntdeck/internal/config"
	"github.com/Thundercloud12/gruntdeck/internal/migrations"
	"github.com/Thundercloud12/gruntdeck/internal/models"
	"github.com/Thundercloud12/gruntdeck/internal/orchestrator"
	"github.com/Thundercloud12/gruntdeck/internal/repository"
	"github.com/Thundercloud12/gruntdeck/internal/ssh"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type FinalStatus struct {
	Target config.Target
	Err    error
}

func main() {
	if len(os.Args) < 2 {
		log.Fatalf("Usage: go run ./cmd/executor <job-id>\nExample: go run ./cmd/executor health-check")
	}
	jobID := os.Args[1]

	ctx := context.Background()
	dbURL := os.Getenv("DATABASE_URL")

	var (
		matchedTargets []config.Target
		activeSteps    []config.Step
		jobName        string
		jobsCfg        *config.JobConfig
		execRepo       repository.ExecutionRepository
		logRepo        repository.LogRepository
		executionID    string
	)

	if dbURL != "" {
		fmt.Println("🐘 Connecting to PostgreSQL database...")
		if err := migrations.RunMigrations(dbURL); err != nil {
			log.Fatalf("Migration failed: %v", err)
		}

		pool, err := pgxpool.New(ctx, dbURL)
		if err != nil {
			log.Fatalf("Failed to connect to Postgres: %v", err)
		}
		defer pool.Close()

		jobRepo := repository.NewPostgresJobRepository(pool)
		invRepo := repository.NewPostgresInventoryRepository(pool)
		execRepo = repository.NewPostgresExecutionRepository(pool)
		logRepo = repository.NewPostgresLogRepository(pool)

		dbJob, err := jobRepo.GetJobByID(ctx, jobID)
		if err != nil {
			log.Fatalf("Error loading job from database: %v", err)
		}
		jobName = dbJob.Name

		dbTargets, err := invRepo.GetTargetByTags(ctx, dbJob.TargetFilter)
		if err != nil {
			log.Fatalf("Error loading targets from database: %v", err)
		}
		for _, dt := range dbTargets {
			portInt, _ := strconv.Atoi(dt.Port)
			if portInt == 0 {
				portInt = 22
			}
			matchedTargets = append(matchedTargets, config.Target{
				Host:    dt.Host,
				Port:    portInt,
				User:    dt.User,
				KeyPath: dt.KeyPath,
				Tags:    dt.Tags,
			})
		}

		for _, step := range dbJob.Steps {
			var cfgStep config.Step
			cfgStep.Type = step.Type

			if len(step.Atrributes) > 0 {
				var attrMap map[string]interface{}
				if err := json.Unmarshal(step.Atrributes, &attrMap); err == nil {
					if v, ok := attrMap["value"].(string); ok {
						cfgStep.Value = v
					}
					if s, ok := attrMap["source_path"].(string); ok {
						cfgStep.SourcePath = s
					}
					if d, ok := attrMap["dest_path"].(string); ok {
						cfgStep.DestPath = d
					}
					if j, ok := attrMap["job_id"].(string); ok {
						cfgStep.JobID = j
					}
					if args, ok := attrMap["args"].([]interface{}); ok {
						for _, a := range args {
							if strArg, ok := a.(string); ok {
								cfgStep.Args = append(cfgStep.Args, strArg)
							}
						}
					}
				}
			}
			activeSteps = append(activeSteps, cfgStep)
		}

		executionID = uuid.New().String()
		err = execRepo.CreateExecution(ctx, models.Execution{
			ID:           executionID,
			JobID:        jobID,
			Status:       "running",
			StartedAt:    time.Now(),
			TargetsTotal: len(matchedTargets),
		})
		if err != nil {
			log.Printf("Warning: Failed to log execution start in database: %v", err)
		}

	} else {
		inventoryCfg, err := config.Load("inventory.yaml")
		if err != nil {
			log.Fatalf("Error loading inventory: %v", err)
		}

		var loadErr error
		jobsCfg, loadErr = config.LoadJobs("jobs.yaml")
		if loadErr != nil {
			log.Fatalf("Error loading jobs: %v", loadErr)
		}

		var activeJob *config.Jobs
		for _, job := range jobsCfg.Jobs {
			if job.ID == jobID {
				activeJob = &job
				break
			}
		}
		if activeJob == nil {
			log.Fatalf("Job '%s' not found in jobs.yaml", jobID)
		}

		jobName = activeJob.Name
		matchedTargets = orchestrator.MatchTargets(inventoryCfg.Target, activeJob.TargetFilter)
		activeSteps = activeJob.Steps
	}

	if len(matchedTargets) == 0 {
		log.Fatalf("No targets matched the filter criteria")
	}

	fmt.Printf("Job: %s | Matching Nodes: %d\n", jobName, len(matchedTargets))
	fmt.Println(strings.Repeat("=", 60))

	results := make(chan FinalStatus, len(matchedTargets))
	var wg sync.WaitGroup

	for _, target := range matchedTargets {
		wg.Add(1)
		go func(t config.Target) {
			defer wg.Done()

			for i, step := range activeSteps {
				err := executeStep(ctx, t, step, jobsCfg, logRepo, executionID)
				if err != nil {
					results <- FinalStatus{Target: t, Err: fmt.Errorf("step %d failed: %w", i+1, err)}
					return
				}
			}
			results <- FinalStatus{Target: t, Err: nil}
		}(target)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	var failed, success int
	for res := range results {
		if res.Err != nil {
			fmt.Printf("❌ [SYSTEM] %s@%s failed: %v\n", res.Target.User, res.Target.Host, res.Err)
			failed++
		} else {
			success++
		}
	}

	fmt.Println(strings.Repeat("=", 60))
	fmt.Printf("Execution Summary: %d Succeeded | %d Failed\n", success, failed)

	if execRepo != nil && executionID != "" {
		status := "succeeded"
		if failed > 0 {
			status = "failed"
		}
		_ = execRepo.UpdateExecution(ctx, models.Execution{
			ID:               executionID,
			JobID:            jobID,
			Status:           status,
			EndedAt:          time.Now(),
			TargetsTotal:     len(matchedTargets),
			TargetsSucceeded: success,
			TargetsFailed:    failed,
		})
	}

	if failed > 0 {
		os.Exit(1)
	}
}

func executeStep(ctx context.Context, t config.Target, step config.Step, jobsCfg *config.JobConfig, logRepo repository.LogRepository, executionID string) error {
	msg := ""
	switch step.Type {
	case "command":
		if step.Value == "" {
			return fmt.Errorf("missing command value")
		}
		err := ssh.RunCommand(ctx, t, step.Value)
		recordLog(ctx, logRepo, executionID, t.Host, step.Type, "info", fmt.Sprintf("Command: %s", step.Value), err)
		return err

	case "script":
		if step.SourcePath == "" {
			return fmt.Errorf("missing script source_path")
		}
		msg = fmt.Sprintf("[%s@%s] 📜 Uploading and executing script %s %v...", t.User, t.Host, step.SourcePath, step.Args)
		fmt.Println(msg)
		err := ssh.RunScript(ctx, t, step.SourcePath, step.Args)
		recordLog(ctx, logRepo, executionID, t.Host, step.Type, "info", msg, err)
		return err

	case "file-copy":
		if step.SourcePath == "" || step.DestPath == "" {
			return fmt.Errorf("missing file-copy source_path or dest_path")
		}
		msg = fmt.Sprintf("[%s@%s] 📁 Copying local %s to remote %s...", t.User, t.Host, step.SourcePath, step.DestPath)
		fmt.Println(msg)
		err := ssh.CopyFile(ctx, t, step.SourcePath, step.DestPath)
		if err == nil {
			successMsg := fmt.Sprintf("[%s@%s] 📁 Successfully copied %s", t.User, t.Host, step.DestPath)
			fmt.Println(successMsg)
			recordLog(ctx, logRepo, executionID, t.Host, step.Type, "info", successMsg, nil)
		} else {
			recordLog(ctx, logRepo, executionID, t.Host, step.Type, "error", msg, err)
		}
		return err

	case "job-ref":
		if step.JobID == "" {
			return fmt.Errorf("missing job-ref job_id")
		}
		msg = fmt.Sprintf("[%s@%s] 🔗 Invoking job reference: %s", t.User, t.Host, step.JobID)
		fmt.Println(msg)
		recordLog(ctx, logRepo, executionID, t.Host, step.Type, "info", msg, nil)
		if jobsCfg != nil {
			return executeJobRef(ctx, t, step.JobID, jobsCfg, logRepo, executionID)
		}
		return nil

	default:
		return fmt.Errorf("unknown step type: %s", step.Type)
	}
}

func executeJobRef(ctx context.Context, t config.Target, jobID string, jobsCfg *config.JobConfig, logRepo repository.LogRepository, executionID string) error {
	var refJob *config.Jobs
	for _, j := range jobsCfg.Jobs {
		if j.ID == jobID {
			refJob = &j
			break
		}
	}
	if refJob == nil {
		return fmt.Errorf("job reference '%s' not found", jobID)
	}

	for i, s := range refJob.Steps {
		err := executeStep(ctx, t, s, jobsCfg, logRepo, executionID)
		if err != nil {
			return fmt.Errorf("under job-ref '%s' step %d: %w", jobID, i+1, err)
		}
	}
	return nil
}

func recordLog(ctx context.Context, logRepo repository.LogRepository, executionID, targetHost, stepID, level, msg string, err error) {
	if logRepo == nil || executionID == "" {
		return
	}
	if err != nil {
		level = "error"
		msg = fmt.Sprintf("%s | Error: %v", msg, err)
	}
	_ = logRepo.AddLogEntry(ctx, models.LogEntry{
		ID:          uuid.New().String(),
		ExecutionID: executionID,
		TargetID:    targetHost,
		StepID:      stepID,
		Timestamp:   time.Now(),
		Level:       level,
		Message:     msg,
	})
}
