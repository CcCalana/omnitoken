//go:build e2e

package e2e

import (
	"os"
	"os/exec"
	"testing"
)

func TestL2Runner(t *testing.T) {
	required := []string{
		"OMNITOKEN_GATEWAY_URL",
		"OMNITOKEN_ADMIN_URL",
		"OMNITOKEN_ADMIN_TOKEN",
		"OMNITOKEN_TEST_DATABASE_URL",
	}
	for _, key := range required {
		if os.Getenv(key) == "" {
			t.Skipf("%s not set", key)
		}
	}
	cmd := exec.Command("go", "run", "./cmd/e2e-runner")
	cmd.Dir = "../.."
	cmd.Env = os.Environ()
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("l2 runner failed: %v\n%s", err, string(output))
	}
}
