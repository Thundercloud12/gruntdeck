package ssh

import (
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

const (
	knownHostsDir  = ".gruntdeck"
	knownHostsFile = "known_hosts"
)


func GetKnownHostsPath() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get working directory: %w", err)
	}

	dirPath := filepath.Join(wd, knownHostsDir)
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		return "", fmt.Errorf("failed to create %s directory: %w", dirPath, err)
	}

	filePath := filepath.Join(dirPath, knownHostsFile)
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		file, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY, 0600)
		if err != nil {
			return "", fmt.Errorf("failed to create empty known_hosts file: %w", err)
		}
		file.Close()
	}

	return filePath, nil
}

func GetHostKeyCallback() (ssh.HostKeyCallback, error) {
	path, err := GetKnownHostsPath()
	if err != nil {
		return nil, err
	}

	khCallback, err := knownhosts.New(path)
	if err != nil {
		return nil, fmt.Errorf("failed to load known_hosts: %w", err)
	}

	return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		err := khCallback(hostname, remote, key)
		if err == nil {
			return nil
		}

		var keyErr *knownhosts.KeyError
		if errors.As(err, &keyErr) {
			normHost := knownhosts.Normalize(hostname)
			if len(keyErr.Want) == 0 {
				return fmt.Errorf("Unknown host key for %s.\nTo trust this host, run:\n    gruntdeck trust %s", hostname, normHost)
			}
			return fmt.Errorf("🚨 SECURITY WARNING: Host key mismatch detected for %s! Possible Man-in-the-Middle attack.", hostname)
		}

		return err
	}, nil
}


func ScanHostKey(hostPort string) (ssh.PublicKey, error) {
	if !strings.Contains(hostPort, ":") {
		hostPort = hostPort + ":22"
	}

	var hostKey ssh.PublicKey
	config := &ssh.ClientConfig{
		User: "keyscan",
		Auth: []ssh.AuthMethod{},
		HostKeyCallback: func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			hostKey = key
			return errors.New("key_captured_abort") 
		},
		Timeout: 5 * time.Second,
	}

	_, err := ssh.Dial("tcp", hostPort, config)
	if err != nil && !strings.Contains(err.Error(), "key_captured_abort") {
		return nil, fmt.Errorf("failed to scan host %s: %w", hostPort, err)
	}

	if hostKey == nil {
		return nil, fmt.Errorf("could not capture host key for %s", hostPort)
	}

	return hostKey, nil
}

func FingerprintSHA256(key ssh.PublicKey) string {
	h := sha256.Sum256(key.Marshal())
	return "SHA256:" + base64.RawStdEncoding.EncodeToString(h[:])
}

func AddHostKey(hostPort string, key ssh.PublicKey) error {
	if !strings.Contains(hostPort, ":") {
		hostPort = hostPort + ":22"
	}

	path, err := GetKnownHostsPath()
	if err != nil {
		return err
	}

	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("failed to open known_hosts for writing: %w", err)
	}
	defer file.Close()

	normalized := knownhosts.Normalize(hostPort)
	line := fmt.Sprintf("%s %s", normalized, ssh.MarshalAuthorizedKey(key))

	if _, err := file.WriteString(line); err != nil {
		return fmt.Errorf("failed to write key to known_hosts: %w", err)
	}

	return nil
}
