package ssh

import (
	"fmt"
	"os"

	"golang.org/x/crypto/ssh"
)

func PublicKeyFile(file string) (ssh.AuthMethod,error) {

	buffer, err := os.ReadFile(file)
	if err != nil {
		return nil, fmt.Errorf("cannot read SSH key file %s: %w", file, err)
	}

	key, err := ssh.ParsePrivateKey(buffer)
	if err != nil {
		return nil, fmt.Errorf("cannot parse SSH key %s: %w", file, err)
	}

	return ssh.PublicKeys(key), nil
	
}