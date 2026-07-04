package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	_"time"

	"github.com/Thundercloud12/gruntdeck/internal/config"
	"github.com/Thundercloud12/gruntdeck/internal/ssh"
)

type FinalStatus struct {
	Target config.Target
	Err    error
}

func main() {
	cfg, err := config.Load("inventory.yaml")
	if err != nil {
		log.Fatalf("Error loading inventory: %v", err)
	}

	if len(os.Args) < 2 {
		log.Fatalf("Usage: go run ./cmd/executor <command>")
	}
	command := strings.Join(os.Args[1:], " ")

	fmt.Printf("Starting LIVE CONCURRENT execution on %d targets...\n", len(cfg.Target))
	fmt.Println(strings.Repeat("=", 60))

	
	ctx := context.Background()

	results := make(chan FinalStatus, len(cfg.Target))
	var wg sync.WaitGroup

	
	for _, target := range cfg.Target {
		wg.Add(1)
		go func(t config.Target) {
			defer wg.Done()
			
			
			err := ssh.RunCommand(ctx, t, command)
			
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