package crypto

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnvelopeEncryptDecrypt(t *testing.T) {
	key := bytes.Repeat([]byte{0x11}, MasterKeySize)
	env, err := NewEnvelope(key)
	if err != nil {
		t.Fatalf("new envelope: %v", err)
	}
	plaintext := []byte("ark-secret-value")

	ciphertext, err := env.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	if bytes.Contains(ciphertext, plaintext) {
		t.Fatalf("ciphertext contains plaintext: %x", ciphertext)
	}
	got, err := env.Decrypt(ciphertext)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if string(got) != string(plaintext) {
		t.Fatalf("plaintext = %q", got)
	}
}

func TestEnvelopeRejectsWrongMasterKey(t *testing.T) {
	env, err := NewEnvelope(bytes.Repeat([]byte{0x11}, MasterKeySize))
	if err != nil {
		t.Fatalf("new envelope: %v", err)
	}
	ciphertext, err := env.Encrypt([]byte("ark-secret-value"))
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}

	wrong, err := NewEnvelope(bytes.Repeat([]byte{0x22}, MasterKeySize))
	if err != nil {
		t.Fatalf("new wrong envelope: %v", err)
	}
	if _, err := wrong.Decrypt(ciphertext); err == nil {
		t.Fatal("expected decrypt with wrong key to fail")
	}
}

func TestEnvelopeRejectsTamperedTag(t *testing.T) {
	env, err := NewEnvelope(bytes.Repeat([]byte{0x11}, MasterKeySize))
	if err != nil {
		t.Fatalf("new envelope: %v", err)
	}
	ciphertext, err := env.Encrypt([]byte("ark-secret-value"))
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	ciphertext[len(ciphertext)-1] ^= 0xff
	if _, err := env.Decrypt(ciphertext); err == nil {
		t.Fatal("expected tampered ciphertext to fail")
	}
}

func TestEnvelopeDoesNotReuseNonce(t *testing.T) {
	env, err := NewEnvelope(bytes.Repeat([]byte{0x11}, MasterKeySize))
	if err != nil {
		t.Fatalf("new envelope: %v", err)
	}
	first, err := env.Encrypt([]byte("same-plaintext"))
	if err != nil {
		t.Fatalf("first encrypt: %v", err)
	}
	second, err := env.Encrypt([]byte("same-plaintext"))
	if err != nil {
		t.Fatalf("second encrypt: %v", err)
	}
	nonceSize := env.aead.NonceSize()
	if bytes.Equal(first[:nonceSize], second[:nonceSize]) {
		t.Fatalf("nonce reused: %x", first[:nonceSize])
	}
}

func TestDecodeMasterKeyHex(t *testing.T) {
	key, err := DecodeMasterKeyHex(strings.Repeat("0a", MasterKeySize))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(key) != MasterKeySize {
		t.Fatalf("key length = %d", len(key))
	}
	if _, err := DecodeMasterKeyHex(""); !errors.Is(err, ErrMasterKeyMissing) {
		t.Fatalf("expected missing key, got %v", err)
	}
	if _, err := DecodeMasterKeyHex("bad"); !errors.Is(err, ErrInvalidMasterKey) {
		t.Fatalf("expected invalid key, got %v", err)
	}
}

func TestLoadMasterKeyPrefersFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "master-key")
	fileKey := strings.Repeat("0b", MasterKeySize)
	if err := os.WriteFile(path, []byte(fileKey+"\n"), 0o600); err != nil {
		t.Fatalf("write key file: %v", err)
	}

	key, err := LoadMasterKey(path, strings.Repeat("0c", MasterKeySize))
	if err != nil {
		t.Fatalf("load key: %v", err)
	}
	if key[0] != 0x0b {
		t.Fatalf("expected file key to win, got %x", key[0])
	}
}

func TestLoadMasterKeyReadError(t *testing.T) {
	_, err := LoadMasterKey(filepath.Join(t.TempDir(), "missing"), strings.Repeat("0c", MasterKeySize))
	if err == nil {
		t.Fatal("expected read error")
	}
}

func TestEnvelopeRejectsShortCiphertext(t *testing.T) {
	env, err := NewEnvelope(bytes.Repeat([]byte{0x11}, MasterKeySize))
	if err != nil {
		t.Fatalf("new envelope: %v", err)
	}
	if _, err := env.Decrypt([]byte("short")); err == nil {
		t.Fatal("expected short ciphertext error")
	}
}

func TestEnvelopeRejectsInvalidInitialization(t *testing.T) {
	if _, err := NewEnvelope([]byte("short")); !errors.Is(err, ErrInvalidMasterKey) {
		t.Fatalf("expected invalid master key, got %v", err)
	}
	var env *Envelope
	if _, err := env.Encrypt([]byte("x")); err == nil {
		t.Fatal("expected nil envelope encrypt error")
	}
	if _, err := env.Decrypt([]byte("x")); err == nil {
		t.Fatal("expected nil envelope decrypt error")
	}
}
