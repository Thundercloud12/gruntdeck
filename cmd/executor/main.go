package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/Thundercloud12/gruntdeck/internal/config"
	
	"github.com/Thundercloud12/gruntdeck/internal/ssh"
)

func main(){
	cfg, err := config.Load("inventory.yaml")
	if err != nil {
		log.Fatalf("Error loading inventory: %v", err)
	}

	if len(os.Args) < 2 {
		log.Fatalf("Usage: %s <command>\nExample: %s \"uptime\"", os.Args[0], os.Args[0])
	}
	command := strings.Join(os.Args[1:], " ")

	fmt.Printf("Loaded %d targets. Starting sequential execution...\n", len(cfg.Target))
	fmt.Printf("Command to execute: %s\n", command)
	fmt.Println(strings.Repeat("-", 40))

	// 3. Loop through targets sequentially
	for _, target := range cfg.Target {
		fmt.Printf("➜ Targeting %s@%s:%d...\n", target.User, target.Host, target.Port)
		
		output, err := ssh.RunCommand(target, command)
		if err != nil {
			fmt.Printf("❌ ERROR: %v\n", err)
		} else {
			// Print the successful output
			fmt.Printf("✅ SUCCESS:\n%s", string(output))
		}
		
		fmt.Println(strings.Repeat("-", 40))
	}
	
	fmt.Println("Execution complete.")
}