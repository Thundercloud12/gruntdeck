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

	"github.com/Thundercloud12/gruntdeck/internal/models"
	"golang.org/x/crypto/ssh"
)

func getAddress(target models.Target) string {
	port := target.Port
	if port == "" {
		port = "22"
	}
	return fmt.Sprintf("%s:%s", target.Host, port)
}

// PublicKeyBytes creates an ssh.AuthMethod from in-memory private key bytes.
func PublicKeyBytes(keyBytes []byte) (ssh.AuthMethod, error) {
	signer, err := ssh.ParsePrivateKey(keyBytes)
	if err != nil {
		return nil, fmt.Errorf("unable to parse in-memory private key: %w", err)
	}
	return ssh.PublicKeys(signer), nil
}

// ResolveAuthMethod resolves authentication using decrypted credential bytes or target key_path fallback.
func ResolveAuthMethod(target models.Target, secretBytes []byte, secretType string) (ssh.AuthMethod, error) {
	if len(secretBytes) > 0 {
		switch strings.ToLower(secretType) {
		case "password":
			return ssh.Password(string(secretBytes)), nil
		default: // "ssh_key"
			return PublicKeyBytes(secretBytes)
		}
	}

	if target.KeyPath != "" {
		return PublicKeyFile(target.KeyPath)
	}

	return nil, fmt.Errorf("no valid SSH credential provided for target %s", target.Host)
}

func RunCommand(ctx context.Context, target models.Target, cmd string) error {
	return RunCommandWithCredential(ctx, target, cmd, nil, nil)
}

func RunCommandWithOutput(ctx context.Context, target models.Target, cmd string, onLogLine func(line string, isErr bool)) error {
	return RunCommandWithCredential(ctx, target, cmd, nil, onLogLine)
}

func RunCommandWithCredential(ctx context.Context, target models.Target, cmd string, cred *models.Credential, onLogLine func(line string, isErr bool)) error {
	var secretBytes []byte
	var secretType string
	if cred != nil {
		secretBytes = cred.EncryptedData // Expecting already decrypted payload or passed in
		secretType = cred.Type
	}

	authMethod, err := ResolveAuthMethod(target, secretBytes, secretType)
	if err != nil {
		return err
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

	address := getAddress(target)

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
			text := scanner.Text()
			if isErr {
				fmt.Printf("%s ❌ %s\n", prefix, text)
			} else {
				fmt.Printf("%s ➜ %s\n", prefix, text)
			}
			if onLogLine != nil {
				onLogLine(text, isErr)
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
func CopyFile(ctx context.Context, target models.Target, localPath string, destPath string) error {
	return CopyFileWithCredential(ctx, target, localPath, destPath, nil)
}

func CopyFileWithCredential(ctx context.Context, target models.Target, localPath string, destPath string, cred *models.Credential) error {
	var secretBytes []byte
	var secretType string
	if cred != nil {
		secretBytes = cred.EncryptedData
		secretType = cred.Type
	}

	authMethod, err := ResolveAuthMethod(target, secretBytes, secretType)
	if err != nil {
		return err
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

	address := getAddress(target)

	client, err := ssh.Dial("tcp", address, sshConfig)
	if err != nil {
		return fmt.Errorf("failed to connect to %s: %w", address, err)
	}
	defer client.Close()

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
func RunScript(ctx context.Context, target models.Target, localPath string, args []string) error {
	return RunScriptWithOutput(ctx, target, localPath, args, nil)
}

func RunScriptWithOutput(ctx context.Context, target models.Target, localPath string, args []string, onLogLine func(line string, isErr bool)) error {
	return RunScriptWithCredential(ctx, target, localPath, args, nil, onLogLine)
}

func RunScriptWithCredential(ctx context.Context, target models.Target, localPath string, args []string, cred *models.Credential, onLogLine func(line string, isErr bool)) error {
	tempDest := fmt.Sprintf("/tmp/gruntdeck_%d.sh", time.Now().UnixNano())

	err := CopyFileWithCredential(ctx, target, localPath, tempDest, cred)
	if err != nil {
		return fmt.Errorf("failed to copy script to remote host: %w", err)
	}

	defer func() {
		_ = RunCommandWithCredential(ctx, target, fmt.Sprintf("rm -f '%s'", tempDest), cred, nil)
	}()

	err = RunCommandWithCredential(ctx, target, fmt.Sprintf("chmod +x '%s'", tempDest), cred, nil)
	if err != nil {
		return fmt.Errorf("failed to make remote script executable: %w", err)
	}

	cmd := tempDest
	if len(args) > 0 {
		var escapedArgs []string
		for _, arg := range args {
			escapedArgs = append(escapedArgs, fmt.Sprintf("'%s'", strings.ReplaceAll(arg, "'", "'\\''")))
		}
		cmd = fmt.Sprintf("%s %s", tempDest, strings.Join(escapedArgs, " "))
	}

	return RunCommandWithCredential(ctx, target, cmd, cred, onLogLine)
}
