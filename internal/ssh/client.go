package ssh

import (
	"bufio"
	"context"
	"fmt"
	"io"
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

	sshConfig := &ssh.ClientConfig{
		User: target.User,
		Auth: []ssh.AuthMethod{
			authMethod,
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         5 * time.Second,
	}

	address := fmt.Sprintf("%s:%d", target.Host, target.Port)

	client, err := ssh.Dial("tcp", address, sshConfig)
	if err != nil {
		return fmt.Errorf("failed to connect to %s: %w", address, err)
	}
	defer client.Close()

	
	go func() {
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
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
				case <-ctx.Done():
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

		// Increase scanner buffer (default is only 64KB)
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