package secrets_test

import (
	"bytes"
	"testing"

	"github.com/Thundercloud12/gruntdeck/internal/secrets"
)

func TestEncryptDecrypt(t *testing.T) {
	svc := secrets.NewServiceWithKey("test-secret-master-key-12345")
	originalSecret := []byte("-----BEGIN OPENSSH PRIVATE KEY-----\ntestkeydata\n-----END OPENSSH PRIVATE KEY-----")

	ciphertext, nonce, err := svc.Encrypt(originalSecret)
	if err != nil {
		t.Fatalf("Failed to encrypt: %v", err)
	}

	if bytes.Equal(originalSecret, ciphertext) {
		t.Errorf("Ciphertext matches plaintext, encryption failed")
	}

	decrypted, err := svc.Decrypt(ciphertext, nonce)
	if err != nil {
		t.Fatalf("Failed to decrypt: %v", err)
	}

	if !bytes.Equal(originalSecret, decrypted) {
		t.Errorf("Decrypted payload '%s' does not match original '%s'", string(decrypted), string(originalSecret))
	}
}

func TestDecryptWithWrongKey(t *testing.T) {
	svcA := secrets.NewServiceWithKey("key-a")
	svcB := secrets.NewServiceWithKey("key-b")

	originalSecret := []byte("super-secret-password")

	ciphertext, nonce, err := svcA.Encrypt(originalSecret)
	if err != nil {
		t.Fatalf("Failed to encrypt: %v", err)
	}

	_, err = svcB.Decrypt(ciphertext, nonce)
	if err == nil {
		t.Errorf("Expected decryption error when using wrong key, got nil")
	}
}
