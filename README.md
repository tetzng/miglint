# miglint

`miglint` is a lightweight CLI to lint migration files named for `golang-migrate`. It only inspects files directly under a migrations directory (no recursion) and reports duplicate, naming, and operational rule violations.

## Install

```bash
go install github.com/tetzng/miglint/cmd/miglint@latest
```

## Usage

```bash
miglint -path ./migrations [options]
```

Required:
- `-path` : directory containing migration files

Optional flags:
- `-ext` : extension filter; match final ext (`sql`) or full ext part (`sql.gz`)
- `-enforce-ext` (default: false) : with `-ext`, treat migration-like files (incl. `.up`/`.down` without ext) whose extension differs as errors
- `-no-gaps` (default: false) : require contiguous version numbers
- `-digits` (default: 0) : fix VERSION width; 0 disables the check
- `-require-down` (default: false) : require both up and down for every version
- `-strict-name-match` (default: false) : require NAME and ExtPart to match between up/down of the same version
- `-strict-pattern` (default: false) : error on candidate files (numeric/migration-like, incl. `.up`/`.down` without ext) that don’t match the migrate pattern

## Examples

- Basic lint:

```bash
miglint -path ./db/migrations
```

- Strict SQL-only, no gaps, up/down required:

```bash
miglint -path ./db/migrations \
  -ext sql -enforce-ext=true \
  -require-down=true \
  -strict-name-match=true \
  -no-gaps=true
```
