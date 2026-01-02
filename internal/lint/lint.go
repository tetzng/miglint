package lint

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var (
	migratePattern    = regexp.MustCompile(`^([0-9]+)_(.+)\.(up|down)\.(.+)$`)
	migrationLikeRe   = regexp.MustCompile(`^[0-9]+_.+\.(up|down)\..+`)
	migrationPrefixRe = regexp.MustCompile(`^([0-9]+)_(.+)\.(up|down)(?:\.(.*))?$`)
)

// Config represents lint options.
type Config struct {
	Path            string
	Ext             string
	EnforceExt      bool
	NoGaps          bool
	Digits          int
	RequireDown     bool
	StrictNameMatch bool
	StrictPattern   bool
}

// Migration represents a single parsed migration file.
type Migration struct {
	Path       string
	FileName   string
	VersionStr string
	Version    int64
	NamePart   string
	Direction  string
	ExtPart    string
	FinalExt   string
}

// VersionGroup aggregates migrations by version.
type VersionGroup struct {
	Up   []*Migration
	Down []*Migration
}

// Lint inspects migration files under cfg.Path and returns lint errors (non-fatal)
// or a fatal error (IO/config issues). Caller handles exit codes/output.
func Lint(cfg Config) ([]string, error) {
	if err := ensureDir(cfg.Path); err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(cfg.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to read path: %v", err)
	}

	var lintErrors []string
	versions := make(map[int64]*VersionGroup)
	var versionKeys []int64

	for _, entry := range entries {
		if entry.IsDir() {
			// Only files directly under the specified path are considered.
			continue
		}

		name := entry.Name()
		fullPath := filepath.Join(cfg.Path, name)
		if entry.Type()&os.ModeType != 0 {
			if entry.Type()&os.ModeSymlink == 0 {
				// Skip non-regular files (device/etc.).
				continue
			}
			info, err := os.Stat(fullPath)
			if err != nil {
				lintErrors = append(lintErrors, fmt.Sprintf("failed to stat symlink: %s: %v", fullPath, err))
				continue
			}
			if !info.Mode().IsRegular() {
				// Skip symlinks to non-regular files.
				continue
			}
		}

		finalExt := finalExtension(name)
		prefixMatch := migrationPrefixRe.FindStringSubmatch(name)
		extPartOpt := ""
		hasDirection := false
		if prefixMatch != nil {
			hasDirection = true
			extPartOpt = prefixMatch[4]
		}
		finalExtForMatch := finalExt
		if hasDirection && extPartOpt == "" {
			// Direction (.up/.down) only; treat as no extension.
			finalExtForMatch = ""
		}
		isMigrationLike := migrationLikeRe.MatchString(name)
		matches := migratePattern.FindStringSubmatch(name)

		if cfg.Ext != "" && cfg.EnforceExt && !extMatches(cfg.Ext, finalExtForMatch, extPartOpt) && (matches != nil || isMigrationLike || hasDirection) {
			// Avoid double-reporting when strict-pattern will already flag it as unmatched.
			if matches != nil || !cfg.StrictPattern {
				lintErrors = append(lintErrors, formatExtMismatch(fullPath, cfg.Ext, finalExtForMatch, extPartOpt))
			}
		}

		if matches == nil {
			if isCandidate(name, finalExtForMatch, extPartOpt, isMigrationLike, hasDirection, cfg) {
				lintErrors = append(lintErrors,
					fmt.Sprintf("unmatched file: %s does not match <VERSION>_<NAME>.(up|down).<ext>", fullPath))
			}
			continue
		}

		extPart := matches[4]

		if cfg.Ext != "" && !extMatches(cfg.Ext, finalExtForMatch, extPart) {
			// When enforce_ext is false we ignore; when true we already recorded the error above.
			continue
		}

		versionStr := matches[1]
		namePart := matches[2]
		direction := matches[3]

		version, parseErr := strconv.ParseInt(versionStr, 10, 64)
		if parseErr != nil {
			lintErrors = append(lintErrors, fmt.Sprintf("version parse error in %s: %v", fullPath, parseErr))
			continue
		}

		if cfg.Digits > 0 && len(versionStr) != cfg.Digits {
			lintErrors = append(lintErrors,
				fmt.Sprintf("digits mismatch: %s has VERSION length %d, expected %d", fullPath, len(versionStr), cfg.Digits))
		}

		group := versions[version]
		if group == nil {
			group = &VersionGroup{}
			versions[version] = group
			versionKeys = append(versionKeys, version)
		}

		migration := &Migration{
			Path:       fullPath,
			FileName:   name,
			VersionStr: versionStr,
			Version:    version,
			NamePart:   namePart,
			Direction:  direction,
			ExtPart:    extPart,
			FinalExt:   finalExt,
		}

		if direction == "up" {
			group.Up = append(group.Up, migration)
		} else {
			group.Down = append(group.Down, migration)
		}
	}

	// Duplicates and pairing checks
	sort.Slice(versionKeys, func(i, j int) bool { return versionKeys[i] < versionKeys[j] })
	for _, version := range versionKeys {
		group := versions[version]
		if len(group.Up) > 1 {
			lintErrors = append(lintErrors,
				fmt.Sprintf("duplicate up migrations for version %d: %s", version, joinPaths(group.Up)))
		}
		if len(group.Down) > 1 {
			lintErrors = append(lintErrors,
				fmt.Sprintf("duplicate down migrations for version %d: %s", version, joinPaths(group.Down)))
		}

		if cfg.RequireDown {
			if len(group.Up) > 0 && len(group.Down) == 0 {
				lintErrors = append(lintErrors,
					fmt.Sprintf("missing down migration for version %d", version))
			}
			if len(group.Down) > 0 && len(group.Up) == 0 {
				lintErrors = append(lintErrors,
					fmt.Sprintf("missing up migration for version %d", version))
			}
		}

		if cfg.StrictNameMatch && len(group.Up) == 1 && len(group.Down) == 1 {
			up := group.Up[0]
			down := group.Down[0]
			if up.NamePart != down.NamePart || up.ExtPart != down.ExtPart {
				lintErrors = append(lintErrors,
					fmt.Sprintf("name/ext mismatch for version %d: up=%s, down=%s", version, up.FileName, down.FileName))
			}
		}
	}

	if cfg.NoGaps && len(versionKeys) > 0 {
		for i := 1; i < len(versionKeys); i++ {
			prev := versionKeys[i-1]
			cur := versionKeys[i]
			if cur != prev+1 {
				missingStart := prev + 1
				missingEnd := cur - 1
				if missingStart == missingEnd {
					lintErrors = append(lintErrors, fmt.Sprintf("missing version %d (between %d and %d)", missingStart, prev, cur))
				} else {
					lintErrors = append(lintErrors, fmt.Sprintf("missing versions %d..%d (between %d and %d)", missingStart, missingEnd, prev, cur))
				}
			}
		}
	}

	return lintErrors, nil
}

func ensureDir(dir string) error {
	info, err := os.Stat(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("path does not exist: %s", dir)
		}
		return fmt.Errorf("failed to stat path: %v", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("path is not a directory: %s", dir)
	}
	return nil
}

func finalExtension(name string) string {
	if idx := strings.LastIndexByte(name, '.'); idx != -1 && idx < len(name)-1 {
		return name[idx+1:]
	}
	if strings.HasSuffix(name, ".") {
		return ""
	}
	return ""
}

func isCandidate(name, finalExt, extPart string, migrationLike, hasDirection bool, cfg Config) bool {
	if !cfg.StrictPattern {
		return false
	}

	if cfg.Ext != "" {
		if extMatches(cfg.Ext, finalExt, extPart) {
			return true
		}
		if cfg.EnforceExt && (migrationLike || hasDirection) {
			return true
		}
		return false
	}

	if len(name) > 0 && name[0] >= '0' && name[0] <= '9' {
		return true
	}
	if migrationLike || hasDirection {
		return true
	}
	return false
}

func extMatches(expected, finalExt, extPart string) bool {
	if expected == "" {
		return true
	}
	if strings.Contains(expected, ".") {
		return extPart == expected
	}
	return finalExt == expected
}

func formatExtMismatch(path, expected, finalExt, extPart string) string {
	if strings.Contains(expected, ".") {
		return fmt.Sprintf("extension mismatch: %s has ext part %q, expected %q", path, humanExt(extPart), expected)
	}
	return fmt.Sprintf("extension mismatch: %s has final extension %q (ext part %q), expected %q", path, humanExt(finalExt), humanExt(extPart), expected)
}

func humanExt(s string) string {
	if s == "" {
		return "<none>"
	}
	return s
}

func joinPaths(ms []*Migration) string {
	paths := make([]string, len(ms))
	for i, m := range ms {
		paths[i] = m.Path
	}
	return strings.Join(paths, ", ")
}
