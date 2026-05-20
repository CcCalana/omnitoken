package agent_adapter

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

var errNoBackupFound = errors.New("no agent config backup found")

type Result struct {
	Paths             map[string]string
	BackupPaths       []string
	RestoredFromPaths []string
	ManagedKeys       []string
	Warnings          []string

	// Legacy compatibility fields. T-046 should remove these after the CLI
	// switches to the canonical Paths and slice fields above.
	SettingsPath    string
	ConfigPath      string
	AuthPath        string
	BackupPath      string
	RestoredFrom    string
	ManagedEnvKeys  []string
	ManagedTomlKeys []string
}

func readJSONObject(path string, invalidPrefix string) (map[string]any, bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return map[string]any{}, false, nil
		}
		return nil, false, fmt.Errorf("read %s: %w", filepath.Base(path), err)
	}
	var root map[string]any
	if err := json.Unmarshal(data, &root); err != nil {
		return nil, false, fmt.Errorf("%w: parse %s: %v", ErrInvalidExistingConfig, invalidPrefix, err)
	}
	if root == nil {
		return nil, false, fmt.Errorf("%w: %s root must be a JSON object", ErrInvalidExistingConfig, invalidPrefix)
	}
	return root, true, nil
}

func marshalJSONObject(root map[string]any) ([]byte, error) {
	data, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal JSON object: %w", err)
	}
	return append(data, '\n'), nil
}

func writeJSONAtomic(path string, root map[string]any, perm fs.FileMode) error {
	data, err := marshalJSONObject(root)
	if err != nil {
		return err
	}
	return writeAtomic(path, data, perm)
}

func writeAtomic(path string, data []byte, perm fs.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".*.tmp")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpPath)
		}
	}()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := tmp.Chmod(perm); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("chmod temp file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("sync temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("rename temp file: %w", err)
	}
	cleanup = false
	_ = syncDir(filepath.Dir(path))
	return nil
}

func syncDir(dir string) error {
	f, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer f.Close()
	return f.Sync()
}

func uniqueBackupPath(dir string, filename string, now time.Time) (string, error) {
	base := filepath.Join(dir, filename+"."+now.Format("20060102T150405.000000000Z")+".bak")
	if _, err := os.Stat(base); errors.Is(err, os.ErrNotExist) {
		return base, nil
	} else if err != nil {
		return "", fmt.Errorf("check backup path: %w", err)
	}
	for i := 1; ; i++ {
		candidate := fmt.Sprintf("%s.%03d", base, i)
		if _, err := os.Stat(candidate); errors.Is(err, os.ErrNotExist) {
			return candidate, nil
		} else if err != nil {
			return "", fmt.Errorf("check backup path: %w", err)
		}
	}
}

func latestBackupPath(dir string) (string, error) {
	return latestNamedBackupPath(dir, "settings.json")
}

func latestNamedBackupPath(dir string, filename string) (string, error) {
	matches, err := filepath.Glob(filepath.Join(dir, filename+".*.bak*"))
	if err != nil {
		return "", fmt.Errorf("list backups: %w", err)
	}
	if len(matches) == 0 {
		return "", errNoBackupFound
	}
	sort.Strings(matches)
	return matches[len(matches)-1], nil
}

func copyFile(src string, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	perm := fs.FileMode(0o600)
	if info, statErr := os.Stat(src); statErr == nil {
		perm = info.Mode().Perm()
	}
	return writeAtomic(dst, data, perm)
}

func resolveHome(override string) (string, error) {
	if strings.TrimSpace(override) != "" {
		return filepath.Clean(override), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home dir: %w", err)
	}
	if strings.TrimSpace(home) == "" {
		return "", fmt.Errorf("home dir is empty")
	}
	return home, nil
}

func nowUTC(now func() time.Time) time.Time {
	if now == nil {
		return time.Now().UTC()
	}
	return now().UTC()
}

func nonEmptyStrings(values ...string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}

func paths(values map[string]string) map[string]string {
	out := make(map[string]string, len(values))
	for key, value := range values {
		if value != "" {
			out[key] = value
		}
	}
	return out
}
