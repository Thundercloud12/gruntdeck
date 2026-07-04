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

	
	commandPayload := strings.Join(activeJob.Steps, "\n")

	
	ctx := context.Background() 
	results := make(chan FinalStatus, len(matchedTargets))
	var wg sync.WaitGroup

	for _, target := range matchedTargets {
		wg.Add(1)
		go func(t config.Target) {
			defer wg.Done()
			err := ssh.RunCommand(ctx, t, commandPayload)
			results <- FinalStatus{Target: t, Err: err}
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