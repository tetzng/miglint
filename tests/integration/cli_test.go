package integration

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// These tests exercise the built CLI via "go run" to verify flags/output/exit codes end-to-end.

func TestCLI_NoErrors(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "000001_create.up.sql", "")
	write(t, dir, "000001_create.down.sql", "")

	stdout, stderr, code := runCli(t, "-path", dir)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d (stdout=%q, stderr=%q)", code, stdout, stderr)
	}
	if !strings.Contains(stdout, "migration lint passed") {
		t.Fatalf("expected success message, got stdout=%q", stdout)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
}

func TestCLI_ExtLessReportedWithStrictPattern(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "000001_create.up", "")

	stdout, stderr, code := runCli(t, "-path", dir, "-ext", "sql", "-enforce-ext", "-strict-pattern")
	if code == 0 {
		t.Fatalf("expected non-zero exit code for unmatched ext-less file")
	}
	if !strings.Contains(stderr, "unmatched file") {
		t.Fatalf("expected unmatched error, stderr=%q", stderr)
	}
	if strings.Contains(stderr, "extension mismatch") {
		t.Fatalf("did not expect separate extension mismatch for ext-less .up when strict-pattern is on, stderr=%q", stderr)
	}
	_ = stdout
}

func TestCLI_ExtLeadingDotAccepted(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "000001_create.up.sql", "")
	write(t, dir, "000001_create.down.sql", "")

	stdout, stderr, code := runCli(t, "-path", dir, "-ext", ".sql", "-enforce-ext")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d (stdout=%q, stderr=%q)", code, stdout, stderr)
	}
	if !strings.Contains(stdout, "migration lint passed") {
		t.Fatalf("expected success message, got stdout=%q", stdout)
	}
	if stderr != "" {
		t.Fatalf("expected empty stderr, got %q", stderr)
	}
}

func runCli(t *testing.T, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	cmd := exec.Command("go", append([]string{"run", "./cmd/miglint"}, args...)...)
	cmd.Dir = repoRoot(t)
	var outBuf, errBuf strings.Builder
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return outBuf.String(), errBuf.String(), exitErr.ExitCode()
		}
		t.Fatalf("failed to run cli: %v", err)
	}
	return outBuf.String(), errBuf.String(), 0
}

func repoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	// tests/integration -> repo root is two levels up
	return filepath.Dir(filepath.Dir(wd))
}

func write(t *testing.T, dir, name, contents string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
