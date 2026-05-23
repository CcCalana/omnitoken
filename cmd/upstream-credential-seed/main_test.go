package main

import (
	"io"
	"strings"
	"testing"
)

func TestArkKeysFromEnv(t *testing.T) {
	env := map[string]string{
		"OMNITOKEN_ARK_KEYS":   " key-a, ,key-b ",
		"OMNITOKEN_ARK_KEYS_1": "key-c",
		"OMNITOKEN_ARK_KEYS_3": "key-d",
	}
	got := arkKeysFromEnv(func(key string) string { return env[key] })
	want := []string{"key-a", "key-b", "key-c", "key-d"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("keys = %v want %v", got, want)
	}
}

func TestDeepSeekKeysFromEnv(t *testing.T) {
	env := map[string]string{
		"OMNITOKEN_DEEPSEEK_KEYS":   " ds-a, ,ds-b ",
		"OMNITOKEN_DEEPSEEK_KEYS_1": "ds-c",
	}
	got := keysFromEnv(func(key string) string { return env[key] }, "OMNITOKEN_DEEPSEEK_KEYS")
	want := []string{"ds-a", "ds-b", "ds-c"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("keys = %v want %v", got, want)
	}
}

func TestRunCLINoKeysIsNoopWithoutMasterKey(t *testing.T) {
	var stdout strings.Builder
	var stderr strings.Builder
	code := runCLI(nil, func(string) string { return "" }, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit = %d stderr=%s", code, stderr.String())
	}
	if strings.TrimSpace(stdout.String()) != "loaded 0 upstream credentials" {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestRunCLIInvalidMasterKeyDoesNotLeakSecret(t *testing.T) {
	env := map[string]string{
		"OMNITOKEN_DATABASE_URL": "postgres://example",
		"OMNITOKEN_MASTER_KEY":   "not-a-valid-secret",
		"OMNITOKEN_ARK_KEYS_1":   "ark-secret-value",
	}
	var stdout strings.Builder
	var stderr strings.Builder
	code := runCLI(nil, func(key string) string { return env[key] }, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit = %d stderr=%s", code, stderr.String())
	}
	combined := stdout.String() + stderr.String()
	for _, secret := range []string{"not-a-valid-secret", "ark-secret-value"} {
		if strings.Contains(combined, secret) {
			t.Fatalf("output leaked secret %q: %s", secret, combined)
		}
	}
}

func TestSortedEnvKeys(t *testing.T) {
	got := sortedEnvKeys(map[string]string{"b": "2", "a": "1"})
	if strings.Join(got, ",") != "a,b" {
		t.Fatalf("sorted keys = %v", got)
	}
}

func TestCredentialAuditSnapshotOmitsSecret(t *testing.T) {
	raw, err := credentialAuditSnapshot("cred-1", "ark", "https://ark.example/v3", "ark-seed-1", 1, "active", "healthy")
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	got := string(raw)
	for _, forbidden := range []string{"encrypted_secret", "secret", "ark-secret-value"} {
		if strings.Contains(got, forbidden) {
			t.Fatalf("snapshot leaked %q: %s", forbidden, got)
		}
	}
	for _, want := range []string{"ark-seed-1", "https://ark.example/v3", "active"} {
		if !strings.Contains(got, want) {
			t.Fatalf("snapshot missing %q: %s", want, got)
		}
	}
}

func TestSeedSQLAuditsCredentialMutations(t *testing.T) {
	for _, want := range []string{
		"INSERT INTO audit_logs",
		"upstream_credential",
		actionCreateUpstreamCredential,
		actionUpdateUpstreamCredential,
		actionDisableUpstreamCredential,
	} {
		joined := insertSeedAuditSQL + insertCredentialSQL + updateCredentialSQL + disableCredentialSQL +
			actionCreateUpstreamCredential + actionUpdateUpstreamCredential + actionDisableUpstreamCredential
		if !strings.Contains(joined, want) {
			t.Fatalf("seed SQL missing %q", want)
		}
	}
	if strings.Contains(insertSeedAuditSQL, "encrypted_secret") {
		t.Fatal("audit SQL should not mention encrypted_secret")
	}
}

var _ io.Writer = (*strings.Builder)(nil)
