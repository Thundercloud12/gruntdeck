package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/Thundercloud12/gruntdeck/internal/ssh"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case "trust":
		if len(os.Args) < 3 {
			fmt.Println("❌ Error: Missing hostname. Usage: gruntdeck trust <hostname>[:port]")
			os.Exit(1)
		}
		runTrust(os.Args[2])

	case "scan-host":
		if len(os.Args) < 3 {
			fmt.Println("❌ Error: Missing hostname. Usage: gruntdeck scan-host <hostname>[:port]")
			os.Exit(1)
		}
		runScanHost(os.Args[2])

	case "help", "-h", "--help":
		printUsage()

	default:
		fmt.Printf("❌ Unknown command: %s\n\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Gruntdeck Host-Key Trust Utility")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  gruntdeck <command> [arguments]")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  trust <host>[:port]      Scan host key, show fingerprint, and prompt to trust it")
	fmt.Println("  scan-host <host>[:port]  Scan host key and trust it automatically (non-interactive)")
	fmt.Println("  help                     Show this help message")
}

func runTrust(hostPort string) {
	fmt.Printf("Connecting to %s...\n", hostPort)
	key, err := ssh.ScanHostKey(hostPort)
	if err != nil {
		fmt.Printf("❌ Failed to scan host key: %v\n", err)
		os.Exit(1)
	}

	fingerprint := ssh.FingerprintSHA256(key)
	fmt.Println(strings.Repeat("-", 50))
	fmt.Printf("Key Type:    %s\n", key.Type())
	fmt.Printf("Fingerprint: %s\n", fingerprint)
	fmt.Println(strings.Repeat("-", 50))

	reader := bufio.NewReader(os.Stdin)
	fmt.Print("Do you trust this host? (yes/no): ")
	input, err := reader.ReadString('\n')
	if err != nil {
		fmt.Printf("❌ Error reading input: %v\n", err)
		os.Exit(1)
	}

	answer := strings.TrimSpace(strings.ToLower(input))
	if answer == "yes" || answer == "y" {
		err = ssh.AddHostKey(hostPort, key)
		if err != nil {
			fmt.Printf("❌ Failed to save host key: %v\n", err)
			os.Exit(1)
		}
		path, _ := ssh.GetKnownHostsPath()
		fmt.Printf("✅ Added %s to trusted hosts (%s)\n", hostPort, path)
	} else {
		fmt.Println("❌ Host rejected. Connection will not be trusted.")
		os.Exit(1)
	}
}

func runScanHost(hostPort string) {
	fmt.Printf("Scanning %s...\n", hostPort)
	key, err := ssh.ScanHostKey(hostPort)
	if err != nil {
		fmt.Printf("❌ Failed to scan host key: %v\n", err)
		os.Exit(1)
	}

	err = ssh.AddHostKey(hostPort, key)
	if err != nil {
		fmt.Printf("❌ Failed to save host key: %v\n", err)
		os.Exit(1)
	}

	path, _ := ssh.GetKnownHostsPath()
	fmt.Printf("✅ Automatically added %s to trusted hosts (%s)\n", hostPort, path)
}
