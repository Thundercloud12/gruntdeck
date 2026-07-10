package ssh

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/Thundercloud12/gruntdeck/internal/config"
	"golang.org/x/crypto/ssh"
)

func RunCommand(ctx context.Context, target config.Target, cmd string) error {

	authMethod, err := PublicKeyFile(target.KeyPath)
	if err != nil {
		return fmt.Errorf("failed to load private key: %w", err)
	}

	hostKeyCallback, err := GetHostKeyCallback()
	if err != nil {
		return fmt.Errorf("failed to setup host key verification: %w", err)
	}

	sshConfig := &ssh.ClientConfig{
		User: target.User,
		Auth: []ssh.AuthMethod{
			authMethod,
		},
		HostKeyCallback: hostKeyCallback,
		Timeout:         5 * time.Second,
	}

	address := fmt.Sprintf("%s:%d", target.Host, target.Port)

	client, err := ssh.Dial("tcp", address, sshConfig)
	if err != nil {
		return fmt.Errorf("failed to connect to %s: %w", address, err)
	}
	defer client.Close()

	keepaliveCtx, cancelKeepalive := context.WithCancel(ctx)
	defer cancelKeepalive()

	go func() {
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-keepaliveCtx.Done():
				return

			case <-ticker.C:
				done := make(chan error, 1)
				go func() {
					_, _, err := client.SendRequest("keepalive@openssh.com", true, nil)
					done <- err
				}()

				select {
				case err := <-done:
					if err != nil {
						_ = client.Close()
						return
					}
				case <-time.After(10 * time.Second):
					_ = client.Close()
					return
				case <-keepaliveCtx.Done():
					return
				}
			}
		}
	}()

	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}
	defer session.Close()

	stdout, err := session.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe: %w", err)
	}

	stderr, err := session.StderrPipe()
	if err != nil {
		return fmt.Errorf("stderr pipe: %w", err)
	}

	prefix := fmt.Sprintf("[%s@%s]", target.User, target.Host)

	var wg sync.WaitGroup

	streamLog := func(reader io.Reader, isErr bool) {
		defer wg.Done()

		scanner := bufio.NewScanner(reader)
		scanner.Buffer(make([]byte, 64*1024), 1024*1024)

		for scanner.Scan() {
			if isErr {
				fmt.Printf("%s ❌ %s\n", prefix, scanner.Text())
			} else {
				fmt.Printf("%s ➜ %s\n", prefix, scanner.Text())
			}
		}

		if err := scanner.Err(); err != nil {
			fmt.Printf("%s scanner error: %v\n", prefix, err)
		}
	}

	wg.Add(2)
	go streamLog(stdout, false)
	go streamLog(stderr, true)

	if err := session.Start(cmd); err != nil {
		return fmt.Errorf("failed to start command: %w", err)
	}

	errCh := make(chan error, 1)

	go func() {
		errCh <- session.Wait()
	}()

	select {

	case <-ctx.Done():
		_ = session.Signal(ssh.SIGTERM)
		_ = client.Close()

		<-errCh
		wg.Wait()

		return fmt.Errorf("execution cancelled: %w", ctx.Err())

	case err := <-errCh:
		wg.Wait()

		if err != nil {
			return fmt.Errorf("command failed: %w", err)
		}

		return nil
	}
}

// CopyFile transfers a local file to a destination path on the remote host.
func CopyFile(ctx context.Context, target config.Target, localPath string, destPath string) error {
	authMethod, err := PublicKeyFile(target.KeyPath)
	if err != nil {
		return fmt.Errorf("failed to load private key: %w", err)
	}

	hostKeyCallback, err := GetHostKeyCallback()
	if err != nil {
		return fmt.Errorf("failed to setup host key verification: %w", err)
	}

	sshConfig := &ssh.ClientConfig{
		User: target.User,
		Auth: []ssh.AuthMethod{
			authMethod,
		},
		HostKeyCallback: hostKeyCallback,
		Timeout:         5 * time.Second,
	}

	address := fmt.Sprintf("%s:%d", target.Host, target.Port)

	client, err := ssh.Dial("tcp", address, sshConfig)
	if err != nil {
		return fmt.Errorf("failed to connect to %s: %w", address, err)
	}
	defer client.Close()

	// Read local file contents
	data, err := os.ReadFile(localPath)
	if err != nil {
		return fmt.Errorf("failed to read local file %s: %w", localPath, err)
	}

	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}
	defer session.Close()

	stdin, err := session.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to get stdin pipe: %w", err)
	}

	// Create parent directories, write the file, and set standard permissions
	cmd := fmt.Sprintf("mkdir -p \"$(dirname '%s')\" && cat > '%s' && chmod 0644 '%s'", destPath, destPath, destPath)
	if err := session.Start(cmd); err != nil {
		return fmt.Errorf("failed to start transfer command: %w", err)
	}

	_, err = stdin.Write(data)
	if err != nil {
		return fmt.Errorf("failed to write data: %w", err)
	}
	stdin.Close()

	if err := session.Wait(); err != nil {
		return fmt.Errorf("file copy failed: %w", err)
	}

	return nil
}

// RunScript copies a local script to the remote host, executes it, and deletes it afterward.
func RunScript(ctx context.Context, target config.Target, localPath string, args []string) error {
	tempDest := fmt.Sprintf("/tmp/gruntdeck_%d.sh", time.Now().UnixNano())

	// 1. Copy the script file to a remote temporary location
	err := CopyFile(ctx, target, localPath, tempDest)
	if err != nil {
		return fmt.Errorf("failed to copy script to remote host: %w", err)
	}

	// Ensure cleanup of the remote script on exit
	defer func() {
		_ = RunCommand(ctx, target, fmt.Sprintf("rm -f '%s'", tempDest))
	}()

	// 2. Make the script executable
	err = RunCommand(ctx, target, fmt.Sprintf("chmod +x '%s'", tempDest))
	if err != nil {
		return fmt.Errorf("failed to make remote script executable: %w", err)
	}

	// 3. Format the execution command with arguments (escaping single quotes)
	cmd := tempDest
	if len(args) > 0 {
		var escapedArgs []string
		for _, arg := range args {
			escapedArgs = append(escapedArgs, fmt.Sprintf("'%s'", strings.ReplaceAll(arg, "'", "'\\''")))
		}
		cmd = fmt.Sprintf("%s %s", tempDest, strings.Join(escapedArgs, " "))
	}

	// 4. Run the script using the production-grade RunCommand
	return RunCommand(ctx, target, cmd)
}