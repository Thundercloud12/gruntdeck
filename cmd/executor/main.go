package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"

	"github.com/Thundercloud12/gruntdeck/internal/config"
	"github.com/Thundercloud12/gruntdeck/internal/orchestrator"
	"github.com/Thundercloud12/gruntdeck/internal/ssh"
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

	inventoryCfg, err := config.Load("inventory.yaml")
	if err != nil {
		log.Fatalf("Error loading inventory: %v", err)
	}

	jobsCfg, err := config.LoadJobs("jobs.yaml")
	if err != nil {
		log.Fatalf("Error loading jobs: %v", err)
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

	matchedTargets := orchestrator.MatchTargets(inventoryCfg.Target, activeJob.TargetFilter)
	if len(matchedTargets) == 0 {
		log.Fatalf("No targets matched the filter: %v", activeJob.TargetFilter)
	}

	fmt.Printf("Job: %s | Matching Nodes: %d\n", activeJob.Name, len(matchedTargets))
	fmt.Println(strings.Repeat("=", 60))

	ctx := context.Background()
	results := make(chan FinalStatus, len(matchedTargets))
	var wg sync.WaitGroup

	for _, target := range matchedTargets {
		wg.Add(1)
		go func(t config.Target) {
			defer wg.Done()
			
			// Execute steps sequentially on this target
			for i, step := range activeJob.Steps {
				err := executeStep(ctx, t, step, jobsCfg)
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

	if failed > 0 {
		os.Exit(1)
	}
}

func executeStep(ctx context.Context, t config.Target, step config.Step, jobsCfg *config.JobConfig) error {
	switch step.Type {
	case "command":
		if step.Value == "" {
			return fmt.Errorf("missing command value")
		}
		return ssh.RunCommand(ctx, t, step.Value)

	case "script":
		if step.SourcePath == "" {
			return fmt.Errorf("missing script source_path")
		}
		fmt.Printf("[%s@%s] 📜 Uploading and executing script %s %v...\n", t.User, t.Host, step.SourcePath, step.Args)
		return ssh.RunScript(ctx, t, step.SourcePath, step.Args)

	case "file-copy":
		if step.SourcePath == "" || step.DestPath == "" {
			return fmt.Errorf("missing file-copy source_path or dest_path")
		}
		fmt.Printf("[%s@%s] 📁 Copying local %s to remote %s...\n", t.User, t.Host, step.SourcePath, step.DestPath)
		err := ssh.CopyFile(ctx, t, step.SourcePath, step.DestPath)
		if err == nil {
			fmt.Printf("[%s@%s] 📁 Successfully copied %s\n", t.User, t.Host, step.DestPath)
		}
		return err

	case "job-ref":
		if step.JobID == "" {
			return fmt.Errorf("missing job-ref job_id")
		}
		fmt.Printf("[%s@%s] 🔗 Invoking job reference: %s\n", t.User, t.Host, step.JobID)
		return executeJobRef(ctx, t, step.JobID, jobsCfg)

	default:
		return fmt.Errorf("unknown step type: %s", step.Type)
	}
}

func executeJobRef(ctx context.Context, t config.Target, jobID string, jobsCfg *config.JobConfig) error {
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
		err := executeStep(ctx, t, s, jobsCfg)
		if err != nil {
			return fmt.Errorf("under job-ref '%s' step %d: %w", jobID, i+1, err)
		}
	}
	return nil
}