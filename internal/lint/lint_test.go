package lint

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLintPassBasic(t *testing.T) {
	dir := t.TempDir()
	touch(t, dir, "000001_create_users.up.sql")
	touch(t, dir, "000001_create_users.down.sql")

	cfg := Config{Path: dir}
	errs, err := Lint(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(errs) != 0 {
		t.Fatalf("expected no lint errors, got %v", errs)
	}
}

func TestRequireDownMissing(t *testing.T) {
	dir := t.TempDir()
	touch(t, dir, "000001_create_users.up.sql")

	cfg := Config{Path: dir, RequireDown: true}
	errs, err := Lint(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertAnyContains(t, errs, "missing down migration for version 1")
}

func TestDuplicateUp(t *testing.T) {
	dir := t.TempDir()
	touch(t, dir, "000001_a.up.sql")
	touch(t, dir, "000001_b.up.sql")

	cfg := Config{Path: dir}
	errs, err := Lint(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertAnyContains(t, errs, "duplicate up migrations for version 1")
}

func TestEnforceExtMismatch(t *testing.T) {
	dir := t.TempDir()
	touch(t, dir, "000001_a.up.sql")
	touch(t, dir, "000001_a.down.txt")

	cfg := Config{Path: dir, Ext: "sql", EnforceExt: true}
	errs, err := Lint(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertAnyContains(t, errs, "extension mismatch")
}

func TestNoGapsTrue(t *testing.T) {
	dir := t.TempDir()
	touch(t, dir, "000001_a.up.sql")
	touch(t, dir, "000003_a.up.sql")

	cfg := Config{Path: dir, NoGaps: true}
	errs, err := Lint(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertAnyContains(t, errs, "missing version 2")
}

func TestStrictNameMatch(t *testing.T) {
	dir := t.TempDir()
	touch(t, dir, "000001_foo.up.sql")
	touch(t, dir, "000001_bar.down.sql")

	cfg := Config{Path: dir, StrictNameMatch: true}
	errs, err := Lint(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertAnyContains(t, errs, "name/ext mismatch")
}

func TestStrictNameMatchSkipsWhenDuplicate(t *testing.T) {
	dir := t.TempDir()
	touch(t, dir, "000001_foo.up.sql")
	touch(t, dir, "000001_bar.up.sql")
	touch(t, dir, "000001_bar.down.sql")

	cfg := Config{Path: dir, StrictNameMatch: true}
	errs, err := Lint(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !anyContains(errs, "duplicate up migrations") {
		t.Fatalf("expected duplicate up error, got %v", errs)
	}
	if anyContains(errs, "name/ext mismatch") {
		t.Fatalf("did not expect name/ext mismatch when duplicates exist: %v", errs)
	}
}

func TestDigitsMismatch(t *testing.T) {
	dir := t.TempDir()
	touch(t, dir, "001_create.up.sql")

	cfg := Config{Path: dir, Digits: 6}
	errs, err := Lint(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertAnyContains(t, errs, "digits mismatch")
}

func TestFailOnUnmatchedCandidate(t *testing.T) {
	dir := t.TempDir()
	// Starts with digits but not matching pattern -> candidate when strict-pattern=true.
	touch(t, dir, "123notes.sql")
	// Non-candidate file should be ignored.
	touch(t, dir, "README.md")

	cfg := Config{Path: dir, StrictPattern: true}
	errs, err := Lint(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertAnyContains(t, errs, "unmatched file")
	if anyContains(errs, "README") {
		t.Fatalf("README.md should not be treated as a candidate: %v", errs)
	}
}

func TestExtPartAndFinalExtModes(t *testing.T) {
	dir := t.TempDir()
	touch(t, dir, "000001_a.up.sql.gz")

	// extPart mode (dot present)
	cfg := Config{Path: dir, Ext: "sql.gz"}
	if errs, err := Lint(cfg); err != nil || len(errs) != 0 {
		t.Fatalf("expected pass with ext=sql.gz, errs=%v, err=%v", errs, err)
	}

	// finalExt mode (no dot)
	cfg = Config{Path: dir, Ext: "gz"}
	if errs, err := Lint(cfg); err != nil || len(errs) != 0 {
		t.Fatalf("expected pass with ext=gz, errs=%v, err=%v", errs, err)
	}

	// mismatch when enforcing final ext
	cfg = Config{Path: dir, Ext: "sql", EnforceExt: true}
	errs, err := Lint(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertAnyContains(t, errs, "extension mismatch")
}

func TestTrailingDotUnmatched(t *testing.T) {
	dir := t.TempDir()
	touch(t, dir, "000001_a.up.")

	cfg := Config{Path: dir, StrictPattern: true}
	errs, err := Lint(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertAnyContains(t, errs, "unmatched file")
}

func TestExtLessUpDownFlaggedWithStrictPattern(t *testing.T) {
	dir := t.TempDir()
	touch(t, dir, "000001_add_user.up")

	cfg := Config{Path: dir, StrictPattern: true, Ext: "sql", EnforceExt: true}
	errs, err := Lint(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertAnyContains(t, errs, "unmatched file")
}

func TestEmptyNameRejectedWithStrictPattern(t *testing.T) {
	dir := t.TempDir()
	touch(t, dir, "000001_.up.sql")

	cfg := Config{Path: dir, StrictPattern: true}
	errs, err := Lint(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertAnyContains(t, errs, "unmatched file")
}

func TestSymlinkedMigrationIsProcessed(t *testing.T) {
	dir := t.TempDir()
	targetDir := t.TempDir()
	target := filepath.Join(targetDir, "target.sql")
	if err := os.WriteFile(target, []byte(""), 0o644); err != nil {
		t.Fatalf("write target %s: %v", target, err)
	}

	link := filepath.Join(dir, "000001_symlink.up.sql")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlink not supported: %v", err)
	}

	cfg := Config{Path: dir, RequireDown: true}
	errs, err := Lint(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertAnyContains(t, errs, "missing down migration for version 1")
}

func touch(t *testing.T, dir, name string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(""), 0o644); err != nil {
		t.Fatalf("write file %s: %v", path, err)
	}
}

func assertAnyContains(t *testing.T, arr []string, substr string) {
	t.Helper()
	if !anyContains(arr, substr) {
		t.Fatalf("expected any string to contain %q, got %v", substr, arr)
	}
}

func anyContains(arr []string, substr string) bool {
	for _, s := range arr {
		if strings.Contains(s, substr) {
			return true
		}
	}
	return false
}
