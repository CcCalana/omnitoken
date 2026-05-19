package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

const defaultMigrationPath = "migrations"

func main() {
	os.Exit(runCLI(os.Args[1:], os.Getenv, os.Stdout, os.Stderr))
}

func runCLI(args []string, getenv func(string) string, stdout io.Writer, stderr io.Writer) int {
	opts := cliOptions{}
	global := flag.NewFlagSet("omnitoken-migrate", flag.ContinueOnError)
	global.SetOutput(stderr)
	global.StringVar(&opts.databaseURL, "database-url", "", "Postgres connection URL; defaults to OMNITOKEN_DATABASE_URL")
	global.StringVar(&opts.path, "path", defaultMigrationPath, "migration directory or file:// URL")

	if err := global.Parse(args); err != nil {
		return 2
	}

	remaining := global.Args()
	if len(remaining) == 0 {
		printUsage(stderr)
		return 2
	}

	if strings.TrimSpace(opts.databaseURL) == "" {
		opts.databaseURL = strings.TrimSpace(getenv("OMNITOKEN_DATABASE_URL"))
	}
	if strings.TrimSpace(opts.databaseURL) == "" {
		fmt.Fprintln(stderr, "missing -database-url or OMNITOKEN_DATABASE_URL")
		return 2
	}

	command := remaining[0]
	commandArgs := remaining[1:]
	switch command {
	case "up":
		return runUp(opts, stdout, stderr)
	case "down":
		return runDown(opts, commandArgs, stdout, stderr)
	case "version":
		return runVersion(opts, stdout, stderr)
	case "force":
		return runForce(opts, commandArgs, stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown command %q\n", command)
		printUsage(stderr)
		return 2
	}
}

type cliOptions struct {
	databaseURL string
	path        string
}

type migrator interface {
	Up() error
	Steps(int) error
	Version() (uint, bool, error)
	Force(int) error
	Close() (sourceErr error, databaseErr error)
}

var openMigrator = func(opts cliOptions) (migrator, error) {
	return newMigrator(opts)
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, "usage: omnitoken-migrate [flags] <up|down|version|force> [command flags]")
	fmt.Fprintln(w, "  down flags:  -steps N      number of versions to roll back; default 1")
	fmt.Fprintln(w, "  force flags: -version N    set schema_migrations version explicitly")
}

func runUp(opts cliOptions, stdout io.Writer, stderr io.Writer) int {
	m, err := openMigrator(opts)
	if err != nil {
		fmt.Fprintf(stderr, "create migrator: %v\n", err)
		return 1
	}
	defer closeMigrator(m, stderr)

	if err := m.Up(); err != nil {
		if errors.Is(err, migrate.ErrNoChange) {
			fmt.Fprintln(stdout, "no change")
			return 0
		}
		fmt.Fprintf(stderr, "migrate up: %v\n", err)
		return 1
	}

	fmt.Fprintln(stdout, "migrate up complete")
	return 0
}

func runDown(opts cliOptions, args []string, stdout io.Writer, stderr io.Writer) int {
	down := flag.NewFlagSet("down", flag.ContinueOnError)
	down.SetOutput(stderr)
	steps := down.Int("steps", 1, "number of versions to roll back")
	if err := down.Parse(args); err != nil {
		return 2
	}
	if *steps <= 0 {
		fmt.Fprintln(stderr, "-steps must be greater than 0")
		return 2
	}

	m, err := openMigrator(opts)
	if err != nil {
		fmt.Fprintf(stderr, "create migrator: %v\n", err)
		return 1
	}
	defer closeMigrator(m, stderr)

	if err := m.Steps(-*steps); err != nil {
		if errors.Is(err, migrate.ErrNoChange) {
			fmt.Fprintln(stdout, "no change")
			return 0
		}
		fmt.Fprintf(stderr, "migrate down: %v\n", err)
		return 1
	}

	fmt.Fprintf(stdout, "migrate down complete: %d step(s)\n", *steps)
	return 0
}

func runVersion(opts cliOptions, stdout io.Writer, stderr io.Writer) int {
	m, err := openMigrator(opts)
	if err != nil {
		fmt.Fprintf(stderr, "create migrator: %v\n", err)
		return 1
	}
	defer closeMigrator(m, stderr)

	version, dirty, err := m.Version()
	if err != nil {
		if errors.Is(err, migrate.ErrNilVersion) {
			fmt.Fprintln(stdout, "version: none dirty: false")
			return 0
		}
		fmt.Fprintf(stderr, "migrate version: %v\n", err)
		return 1
	}

	fmt.Fprintf(stdout, "version: %d dirty: %t\n", version, dirty)
	return 0
}

func runForce(opts cliOptions, args []string, stdout io.Writer, stderr io.Writer) int {
	force := flag.NewFlagSet("force", flag.ContinueOnError)
	force.SetOutput(stderr)
	version := force.Int("version", 0, "schema version to force")
	if err := force.Parse(args); err != nil {
		return 2
	}
	versionSet := false
	force.Visit(func(f *flag.Flag) {
		if f.Name == "version" {
			versionSet = true
		}
	})
	if !versionSet {
		fmt.Fprintln(stderr, "force requires -version")
		return 2
	}

	m, err := openMigrator(opts)
	if err != nil {
		fmt.Fprintf(stderr, "create migrator: %v\n", err)
		return 1
	}
	defer closeMigrator(m, stderr)

	if err := m.Force(*version); err != nil {
		fmt.Fprintf(stderr, "migrate force: %v\n", err)
		return 1
	}

	fmt.Fprintf(stdout, "forced version: %d\n", *version)
	return 0
}

func newMigrator(opts cliOptions) (*migrate.Migrate, error) {
	sourceURL, err := migrationSourceURL(opts.path)
	if err != nil {
		return nil, err
	}

	m, err := migrate.New(sourceURL, opts.databaseURL)
	if err != nil {
		return nil, err
	}
	return m, nil
}

func closeMigrator(m migrator, stderr io.Writer) {
	sourceErr, databaseErr := m.Close()
	if sourceErr != nil {
		fmt.Fprintf(stderr, "close migration source: %v\n", sourceErr)
	}
	if databaseErr != nil {
		fmt.Fprintf(stderr, "close migration database: %v\n", databaseErr)
	}
}

func migrationSourceURL(rawPath string) (string, error) {
	path := strings.TrimSpace(rawPath)
	if path == "" {
		return "", fmt.Errorf("migration path is required")
	}

	if strings.Contains(path, "://") && !strings.HasPrefix(path, "file://") {
		return path, nil
	}
	path = strings.TrimPrefix(path, "file://")
	path = strings.TrimSpace(path)
	if path == "" {
		return "", fmt.Errorf("migration path is required")
	}

	if isWindowsAbsPath(path) {
		return "file://" + slashPath(path), nil
	}

	abs, err := filepath.Abs(filepath.FromSlash(path))
	if err != nil {
		return "", fmt.Errorf("absolute migration path: %w", err)
	}
	return "file://" + slashPath(abs), nil
}

func slashPath(path string) string {
	return strings.ReplaceAll(filepath.ToSlash(path), "\\", "/")
}

func isWindowsAbsPath(path string) bool {
	if len(path) < 3 {
		return false
	}
	drive := path[0]
	return ((drive >= 'A' && drive <= 'Z') || (drive >= 'a' && drive <= 'z')) &&
		path[1] == ':' &&
		(path[2] == '\\' || path[2] == '/')
}
