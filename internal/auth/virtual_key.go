package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base32"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/google/uuid"
)

const (
	VirtualKeySystemPrefix = "omt_"
	virtualKeyPrefixLength = 12
	virtualKeyPrefixBytes  = 7
	virtualKeySecretBytes  = 32
)

var ErrVirtualKeyNotFound = errors.New("virtual key not found")

type PlaintextVirtualKey struct {
	Prefix string
	Secret string
	Token  string
	Hash   []byte
}

type VirtualKeyRecord struct {
	APIKeyID uuid.UUID
	OrgID    uuid.UUID
	UserID   uuid.UUID
	KeyHash  []byte
	Status   string
}

type VirtualKeyStore interface {
	LookupVirtualKey(ctx context.Context, prefix string) (VirtualKeyRecord, error)
}

func GenerateVirtualKey() (PlaintextVirtualKey, error) {
	return GenerateVirtualKeyFromReader(rand.Reader)
}

func GenerateVirtualKeyFromReader(random io.Reader) (PlaintextVirtualKey, error) {
	prefixRaw := make([]byte, virtualKeyPrefixBytes)
	if _, err := io.ReadFull(random, prefixRaw); err != nil {
		return PlaintextVirtualKey{}, fmt.Errorf("generate key prefix: %w", err)
	}
	secretRaw := make([]byte, virtualKeySecretBytes)
	if _, err := io.ReadFull(random, secretRaw); err != nil {
		return PlaintextVirtualKey{}, fmt.Errorf("generate key secret: %w", err)
	}

	prefix := strings.ToLower(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(prefixRaw))
	secret := base64.RawURLEncoding.EncodeToString(secretRaw)
	token := VirtualKeySystemPrefix + prefix + "_" + secret

	return PlaintextVirtualKey{
		Prefix: prefix,
		Secret: secret,
		Token:  token,
		Hash:   HashSecret(secret),
	}, nil
}

func ParseVirtualKey(raw string) (prefix string, secret string, ok bool) {
	key := strings.TrimSpace(raw)
	if !strings.HasPrefix(key, VirtualKeySystemPrefix) {
		return "", "", false
	}

	// Demo-Ready 简化版: split only once after "omt_" so any extra
	// underscores are treated as part of the secret. T-005c will revisit the
	// full key grammar together with timing-safe comparison.
	parts := strings.SplitN(strings.TrimPrefix(key, VirtualKeySystemPrefix), "_", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	prefix, secret = parts[0], parts[1]
	if !validPrefix(prefix) || secret == "" {
		return "", "", false
	}
	decoded, err := base64.RawURLEncoding.DecodeString(secret)
	if err != nil || len(decoded) != virtualKeySecretBytes {
		return "", "", false
	}
	return prefix, secret, true
}

func HashSecret(secret string) []byte {
	sum := sha256.Sum256([]byte(secret))
	return sum[:]
}

func AuthenticateVirtualKey(ctx context.Context, store VirtualKeyStore, raw string) (Subject, bool, error) {
	prefix, secret, ok := ParseVirtualKey(raw)
	if !ok {
		return Subject{}, false, nil
	}

	record, err := store.LookupVirtualKey(ctx, prefix)
	if err != nil {
		if errors.Is(err, ErrVirtualKeyNotFound) {
			return Subject{}, false, nil
		}
		return Subject{}, false, err
	}
	if record.Status != "active" {
		return Subject{}, false, nil
	}

	// Demo-Ready 简化版: byte equality is intentionally not timing-safe.
	// T-005c replaces this with subtle.ConstantTimeCompare.
	if !equalBytes(record.KeyHash, HashSecret(secret)) {
		return Subject{}, false, nil
	}

	return Subject{
		UserID:   record.UserID,
		OrgID:    record.OrgID,
		APIKeyID: record.APIKeyID,
	}, true, nil
}

func validPrefix(prefix string) bool {
	if len(prefix) != virtualKeyPrefixLength {
		return false
	}
	for _, ch := range prefix {
		if (ch < 'a' || ch > 'z') && (ch < '2' || ch > '7') {
			return false
		}
	}
	return true
}

func equalBytes(left []byte, right []byte) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}
