package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

const MasterKeySize = 32

var (
	ErrMasterKeyMissing = errors.New("master key missing")
	ErrInvalidMasterKey = errors.New("invalid master key")
)

type Envelope struct {
	aead cipher.AEAD
	rand io.Reader
}

func NewEnvelope(masterKey []byte) (*Envelope, error) {
	return NewEnvelopeWithRand(masterKey, rand.Reader)
}

func NewEnvelopeWithRand(masterKey []byte, random io.Reader) (*Envelope, error) {
	if len(masterKey) != MasterKeySize {
		return nil, fmt.Errorf("%w: expected %d bytes", ErrInvalidMasterKey, MasterKeySize)
	}
	if random == nil {
		random = rand.Reader
	}
	block, err := aes.NewCipher(masterKey)
	if err != nil {
		return nil, fmt.Errorf("create aes cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create gcm: %w", err)
	}
	return &Envelope{aead: aead, rand: random}, nil
}

func (e *Envelope) Encrypt(plaintext []byte) ([]byte, error) {
	if e == nil || e.aead == nil {
		return nil, errors.New("envelope is not initialized")
	}
	nonce := make([]byte, e.aead.NonceSize())
	if _, err := io.ReadFull(e.rand, nonce); err != nil {
		return nil, fmt.Errorf("generate nonce: %w", err)
	}
	sealed := e.aead.Seal(nil, nonce, plaintext, nil)
	out := make([]byte, 0, len(nonce)+len(sealed))
	out = append(out, nonce...)
	out = append(out, sealed...)
	return out, nil
}

func (e *Envelope) Decrypt(ciphertext []byte) ([]byte, error) {
	if e == nil || e.aead == nil {
		return nil, errors.New("envelope is not initialized")
	}
	nonceSize := e.aead.NonceSize()
	if len(ciphertext) < nonceSize+e.aead.Overhead() {
		return nil, errors.New("ciphertext is too short")
	}
	nonce := ciphertext[:nonceSize]
	sealed := ciphertext[nonceSize:]
	plaintext, err := e.aead.Open(nil, nonce, sealed, nil)
	if err != nil {
		return nil, fmt.Errorf("decrypt envelope: %w", err)
	}
	return plaintext, nil
}

func LoadMasterKey(filePath string, rawHex string) ([]byte, error) {
	path := strings.TrimSpace(filePath)
	if path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read master key file: %w", err)
		}
		return DecodeMasterKeyHex(string(data))
	}
	return DecodeMasterKeyHex(rawHex)
}

func DecodeMasterKeyHex(raw string) ([]byte, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return nil, ErrMasterKeyMissing
	}
	key, err := hex.DecodeString(value)
	if err != nil {
		return nil, fmt.Errorf("%w: hex decode failed", ErrInvalidMasterKey)
	}
	if len(key) != MasterKeySize {
		return nil, fmt.Errorf("%w: expected %d bytes", ErrInvalidMasterKey, MasterKeySize)
	}
	return key, nil
}
