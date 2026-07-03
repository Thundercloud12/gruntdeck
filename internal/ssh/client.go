package ssh

import (
	"fmt"
	"time"


	"github.com/Thundercloud12/gruntdeck/internal/config"
	"golang.org/x/crypto/ssh"
)


func RunCommand(target config.Target, cmd string) ([]byte, error) {
	
	authMethod, err := PublicKeyFile(target.KeyPath)
	if err != nil {
		return nil, fmt.Errorf("auth setup failed: %w", err)
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
		return nil, fmt.Errorf("failed to dial: %w", err)
	}
	defer client.Close()

	
	session, err := client.NewSession()
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}
	defer session.Close()

	
	output, err := session.CombinedOutput(cmd)
	if err != nil {
		return nil, fmt.Errorf("failed to run command: %w\nOutput: %s", err, string(output))
	}

	return output, nil
}