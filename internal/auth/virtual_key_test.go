package auth

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestGenerateVirtualKeyFromReader(t *testing.T) {
	key, err := GenerateVirtualKeyFromReader(bytes.NewReader(bytes.Repeat([]byte{0x42}, virtualKeyPrefixBytes+virtualKeySecretBytes)))
	if err != nil {
		t.Fatalf("generate virtual key: %v", err)
	}
	if !strings.HasPrefix(key.Token, "omt_"+key.Prefix+"_") {
		t.Fatalf("unexpected token format: %q", key.Token)
	}
	if len(key.Prefix) != virtualKeyPrefixLength {
		t.Fatalf("prefix length = %d", len(key.Prefix))
	}
	if len(key.Hash) != 32 {
		t.Fatalf("hash length = %d", len(key.Hash))
	}
	prefix, secret, ok := ParseVirtualKey(key.Token)
	if !ok {
		t.Fatalf("generated token should parse: %q", key.Token)
	}
	if prefix != key.Prefix || secret != key.Secret {
		t.Fatalf("parse mismatch: %q/%q", prefix, secret)
	}
}

func TestGenerateVirtualKeyUsesCryptoRandom(t *testing.T) {
	key, err := GenerateVirtualKey()
	if err != nil {
		t.Fatalf("generate virtual key: %v", err)
	}
	if _, _, ok := ParseVirtualKey(key.Token); !ok {
		t.Fatalf("generated token should parse: %q", key.Token)
	}
}

func TestGenerateVirtualKeyFromReaderReturnsReadError(t *testing.T) {
	if _, err := GenerateVirtualKeyFromReader(errReader{}); err == nil {
		t.Fatal("expected reader error")
	}
}

func TestParseVirtualKeyAllowsSecretUnderscores(t *testing.T) {
	secret := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa_"
	token := "omt_abcdefghijkl_" + secret

	prefix, gotSecret, ok := ParseVirtualKey(token)
	if !ok {
		t.Fatal("expected token to parse")
	}
	if prefix != "abcdefghijkl" || gotSecret != secret {
		t.Fatalf("parse mismatch: prefix=%q secret=%q", prefix, gotSecret)
	}
}

func TestParseVirtualKeyRejectsMalformedTokens(t *testing.T) {
	tests := []string{
		"",
		"sk_abcdefghijkl_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		"omt_short_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		"omt_abcdefghijkl",
		"omt_abcdefghijk!_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		"omt_abcdefghijkl_not-base64",
	}
	for _, token := range tests {
		t.Run(token, func(t *testing.T) {
			if _, _, ok := ParseVirtualKey(token); ok {
				t.Fatalf("expected %q to be rejected", token)
			}
		})
	}
}

func TestAuthenticateVirtualKey(t *testing.T) {
	ctx := context.Background()
	key, err := GenerateVirtualKeyFromReader(bytes.NewReader(bytes.Repeat([]byte{0x77}, virtualKeyPrefixBytes+virtualKeySecretBytes)))
	if err != nil {
		t.Fatalf("generate virtual key: %v", err)
	}
	record := VirtualKeyRecord{
		APIKeyID: uuid.New(),
		OrgID:    uuid.New(),
		UserID:   uuid.New(),
		KeyHash:  key.Hash,
		Status:   "active",
	}

	subject, ok, err := AuthenticateVirtualKey(ctx, fakeStore{record: record}, key.Token)
	if err != nil {
		t.Fatalf("authenticate: %v", err)
	}
	if !ok {
		t.Fatal("expected authentication success")
	}
	if subject.APIKeyID != record.APIKeyID || subject.OrgID != record.OrgID || subject.UserID != record.UserID {
		t.Fatalf("subject mismatch: %#v", subject)
	}
}

func TestAuthenticateVirtualKeyRejectsInvalidDisabledAndMissing(t *testing.T) {
	key, err := GenerateVirtualKeyFromReader(bytes.NewReader(bytes.Repeat([]byte{0x33}, virtualKeyPrefixBytes+virtualKeySecretBytes)))
	if err != nil {
		t.Fatalf("generate virtual key: %v", err)
	}

	tests := []struct {
		name  string
		token string
		store VirtualKeyStore
	}{
		{name: "invalid token", token: "not-a-key", store: fakeStore{}},
		{name: "not found", token: key.Token, store: fakeStore{err: ErrVirtualKeyNotFound}},
		{name: "disabled", token: key.Token, store: fakeStore{record: VirtualKeyRecord{KeyHash: key.Hash, Status: "disabled"}}},
		{name: "hash mismatch", token: key.Token, store: fakeStore{record: VirtualKeyRecord{KeyHash: HashSecret("wrong"), Status: "active"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, ok, err := AuthenticateVirtualKey(context.Background(), tt.store, tt.token); err != nil || ok {
				t.Fatalf("expected auth failure without error, ok=%t err=%v", ok, err)
			}
		})
	}
}

func TestAuthenticateVirtualKeyReturnsStoreError(t *testing.T) {
	key, err := GenerateVirtualKeyFromReader(bytes.NewReader(bytes.Repeat([]byte{0x55}, virtualKeyPrefixBytes+virtualKeySecretBytes)))
	if err != nil {
		t.Fatalf("generate virtual key: %v", err)
	}
	storeErr := errors.New("database offline")

	if _, ok, err := AuthenticateVirtualKey(context.Background(), fakeStore{err: storeErr}, key.Token); !errors.Is(err, storeErr) || ok {
		t.Fatalf("expected store error, ok=%t err=%v", ok, err)
	}
}

func TestEqualBytesRejectsLengthMismatch(t *testing.T) {
	if equalBytes([]byte{1}, []byte{1, 2}) {
		t.Fatal("expected length mismatch to fail")
	}
}

type errReader struct{}

func (errReader) Read([]byte) (int, error) {
	return 0, errors.New("read failed")
}

type fakeStore struct {
	record VirtualKeyRecord
	err    error
}

func (s fakeStore) LookupVirtualKey(context.Context, string) (VirtualKeyRecord, error) {
	if s.err != nil {
		return VirtualKeyRecord{}, s.err
	}
	return s.record, nil
}
