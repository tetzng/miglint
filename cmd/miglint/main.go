package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/tetzng/miglint/internal/lint"
)

func main() {
	cfg, code := parseFlags()
	if code != 0 {
		os.Exit(code)
	}

	lintErrors, err := lint.Lint(*cfg)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}

	if len(lintErrors) > 0 {
		for _, e := range lintErrors {
			fmt.Fprintln(os.Stderr, e)
		}
		os.Exit(1)
	}

	fmt.Println("migration lint passed")
}

func parseFlags() (*lint.Config, int) {
	cfg := &lint.Config{}

	flag.StringVar(&cfg.Path, "path", "", "directory containing migration files (required)")
	flag.StringVar(&cfg.Ext, "ext", "", "extension filter; match final ext (sql) or full ext part (sql.gz)")
	flag.BoolVar(&cfg.EnforceExt, "enforce-ext", false, "with -ext, error when migration-like files (incl. .up/.down) don’t match the ext")
	flag.BoolVar(&cfg.NoGaps, "no-gaps", false, "require contiguous version sequence (no gaps)")
	flag.IntVar(&cfg.Digits, "digits", 0, "fixed number of digits for VERSION (0 disables check)")
	flag.BoolVar(&cfg.RequireDown, "require-down", false, "require both up and down for every version")
	flag.BoolVar(&cfg.StrictNameMatch, "strict-name-match", false, "require up/down to have identical NAME and ExtPart for the same version")
	flag.BoolVar(&cfg.StrictPattern, "strict-pattern", false, "treat candidate but unmatched files as errors")

	flag.Usage = func() {
		if _, err := fmt.Fprintf(flag.CommandLine.Output(), "Usage: %s -path DIR [options]\n", filepath.Base(os.Args[0])); err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
		flag.PrintDefaults()
	}

	flag.Parse()

	if cfg.Ext != "" {
		cfg.Ext = strings.TrimPrefix(cfg.Ext, ".")
	}

	if cfg.Path == "" {
		fmt.Fprintln(os.Stderr, "error: -path is required")
		flag.Usage()
		return nil, 2
	}
	if cfg.EnforceExt && cfg.Ext == "" {
		fmt.Fprintln(os.Stderr, "error: -enforce-ext requires -ext")
		flag.Usage()
		return nil, 2
	}
	if cfg.Digits < 0 {
		fmt.Fprintln(os.Stderr, "error: -digits must be >= 0")
		flag.Usage()
		return nil, 2
	}

	return cfg, 0
}
