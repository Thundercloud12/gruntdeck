package secrets

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
)

type Service struct {
	key []byte
}

func NewService() *Service {
	masterKey := os.Getenv("GRUNTDECK_MASTER_KEY")
	if masterKey == "" {
		log.Fatal("GRUNTDECK_MASTER_KEY environment variable is required (used to encrypt stored credentials)")
	}
	hash := sha256.Sum256([]byte(masterKey))
	return &Service{
		key: hash[:],
	}
}

func NewServiceWithKey(keyString string) *Service {
	hash := sha256.Sum256([]byte(keyString))
	return &Service{
		key: hash[:],
	}
}

func (s *Service) Encrypt(plaintext []byte) ([]byte, []byte, error) {
	if len(plaintext) == 0 {
		return nil, nil, errors.New("cannot encrypt empty plaintext")
	}

	block, err := aes.NewCipher(s.key)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create AES cipher block: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create GCM block: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, nil, fmt.Errorf("failed to generate random nonce: %w", err)
	}

	ciphertext := gcm.Seal(nil, nonce, plaintext, nil)
	return ciphertext, nonce, nil
}

func (s *Service) Decrypt(ciphertext, nonce []byte) ([]byte, error) {
	if len(ciphertext) == 0 || len(nonce) == 0 {
		return nil, errors.New("ciphertext and nonce must not be empty")
	}

	block, err := aes.NewCipher(s.key)
	if err != nil {
		return nil, fmt.Errorf("failed to create AES cipher block: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM block: %w", err)
	}

	if len(nonce) != gcm.NonceSize() {
		return nil, errors.New("invalid nonce size")
	}

	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt ciphertext (invalid key or tampered data): %w", err)
	}

	return plaintext, nil
}
