# format-yaml

[![Go Reference](https://pkg.go.dev/badge/github.com/andrew-grechkin/format-yaml.svg)](https://pkg.go.dev/github.com/andrew-grechkin/format-yaml)

A CLI filter for formatting YAML documents. Highly opinionated but dependency free.

> this is a sibling tool to https://github.com/andrew-grechkin/update-yaml

## SYNOPSIS

```bash
# source from STDIN
format-yaml <<< 'name: old # some name'
```

```bash
# source from file(s), output is STDOUT
format-yaml file.yaml
```

```bash
# source from file(s), overwrite the files in place
format-yaml -i file1.yaml file2.yaml
```

## OPTIONS

- -I               Inplace via truncate (preserves inode; hardlinks keep working; not atomic)
- -i, --inplace    Inplace via atomic temp-file-plus-rename (new inode; hardlinks break)
- -c, --check      Exit non-zero if the input would be reformatted; write nothing
- -d, --diff       Print a unified diff of the reformatting; exit non-zero on change
- -h, --help       Display help message
- -m, --man        Display full readme         (tip: format-yaml --man | colored-md)
- -v, --version    Display version information (tip: format-yaml --version | jq -r .Version)
- --               Stop options processing and treat everything after as positional arguments

(`-i` `--inplace`), `-I`, `--check`, and `--diff` are mutually exclusive.

## ENVIRONMENT

- `FORMAT_YAML_MODE`       - one of `minimal`, `standard`, `full`, `pedantic` (default: `standard`)
- `FORMAT_YAML_LINE_WIDTH` - positive integer, the display column line width enforced by the fold/unfold rules at
  `full+`. Lines whose rendered width exceeds it are unfolded to block; short block sequences that still fit inside it
  are folded to inline flow. If explicit value is not provided the default is 120. For TTY output default is detected as
  (TTY width - 2) but never more than 120.

## INSTALLATION

### Using `mise`

```bash
mise use go:github.com/andrew-grechkin/format-yaml@latest
```

### Building from source

```bash
go install github.com/andrew-grechkin/format-yaml@latest
```

By default, `go install` creates binaries in `$GOBIN` or `$GOPATH/bin`.
To make sure you can use the installed binary you need to add this directory to your path.

```bash
# ensure the go install binaries are in your PATH, consider adding to your shell startup config
export PATH="${GOBIN:-${GOPATH:-$HOME/go}/bin}:$PATH"
```

## FEATURES

- Removes redundant quoting only when the unquoted value round-trips to the same type. YAML 1.1 landmines (`yes` / `no`
  / `on` / `off` booleans; genuine sexagesimal integers like `"1:30:00"` or `"1:2:3:4"`) are always kept quoted so
  downstream YAML 1.1 parsers (Ansible, older Ruby) don't reinterpret them. Docker-compose-shaped strings like `"80:80"`
  are NOT sexagesimal (second section exceeds 59) and unquote cleanly
- Normalizes null literals, indentation, blank-line placement, and multi-line scalars
- Rewrites multi-line strings as block scalars with the smallest lossless chomp (`|-`, `|`, `|+`). Trailing empty lines
  are preserved
- Folds short block sequences to inline flow and unfolds wide or quote-heavy inline collections back to block,
  symmetrically (the "mirror rule": neither direction fires when items would need quotes)
- Transparent BOM handling: strips UTF-8 BOMs, transcodes UTF-16 (BE/LE) and UTF-32 (BE/LE) payloads to UTF-8 on the fly
- Normalizes CRLF line endings to LF
- Round-trip idempotent by construction: `format-yaml < file | format-yaml` produces byte-identical output
- Ships as a single binary, dependency free
- Converts JSON to YAML (side effect)

## USAGE

Formatting is cumulative across four modes, selected via `FORMAT_YAML_MODE`:

- **minimal** - These are MUSTS for any yaml file out there:
  - Emits a `---` document header
  - Strips redundant quotes when it will not change the parsed type
  - Trims trailing whitespaces
  - Ensures a final newline

- **standard** *(default)* - everything in `minimal`, plus:
  - Nulls rendered as the literal `null` (never empty or `~`)
  - Fix indentation
  - Normalises inline-comment spacing between the value and `#`
  - Preserves source blank lines between top-level entries (collapsed to one) and adds a blank whenever either neighbour
    renders as a multi-line value

- **full** - everything in `standard`, plus:
  - Rewrites multiline scalars as block scalars, using the smallest lossless chomp: `|-` for 0 trailing newlines, `|`
    for exactly 1, `|+` for 2 or more
  - Rewrites double-quoted scalars as single-quoted where there is no ambiguity
  - Sorts mapping keys alphabetically at every level. Exception: a mapping whose children touch anchors or aliases keeps
    its source order, because YAML anchors are position-sensitive and any layout the user chose there is deliberate
  - Folds short block sequences into inline flow when every item can be a plain scalar and the folded line fits within
    the configured line width (see `FORMAT_YAML_LINE_WIDTH`). Unfolds inline collections when the line exceeds it OR
    any item requires quoting that can't be dropped (e.g. `'42'`). Neither direction touches block mappings, top-level
    flow, or collections nested inside sequences - the pass is deliberately scoped to the simple, obvious cases only
  - Discards source blank lines between top-level entries and re-inserts them purely from the current AST shape
    (multi-line neighbour → 1 blank; otherwise → 0). Sorting reshuffles entries, so preserving author-placed blank
    positions would mean carrying stale visual grouping into a new context - full+ trades that for a fully deterministic
    layout. If you use blank lines to group related settings and want those preserved or need to keep order,
    stay on `standard`

- **pedantic** - everything in `full`, plus:
  - Emits a `...` document footer

## EXIT CODES

- **0**: Success (write modes wrote successfully; `--check` / `--diff` found no changes)
- **1**: `--check` or `--diff` found the input would be reformatted (the tool ran fine; the result is "not yet formatted")
- **2**: Parse or format error (malformed YAML, unsupported input encoding)
- **3**: I/O error (couldn't read, write, stat, or rename a file)

## GOTCHAS

### [`github.com/goccy/go-yaml`](https://github.com/goccy/go-yaml/pulls) is FULL of bugs and poorly maintained

One can see that there are dozens of pull requests fixing bugs in the library. Maintainers seem just ignore them and
nothing is being fixed for a long time.

This is a bitter irony because author claimed one of the reasons for this library to exist is [poorly maintained](https://github.com/goccy/go-yaml#why-a-new-library) `go-yaml/yaml`.

I'm trying to [workaround some of the bugs](pkg/patch/goccy.go) in my code, but of course something can slip in.

### `--inplace` is atomic; `-I` is inode-preserving

The two variants make different trade-offs:

- `-i` / `--inplace` writes to a temp sibling and `rename(2)`s over the target. The rename is atomic on POSIX
  filesystems, so a crash never leaves a half-written file. Side effect: the target gets a new inode, and any
  hardlinks pointing at the old inode continue to see the pre-format content. Use this when atomicity matters more
  than hardlink preservation (the common case)
- `-I` truncates and rewrites in place, keeping the file's inode so hardlinks stay attached to the updated content.
  Trade-off: not atomic - a crash or power loss between the truncate and the completion of the write leaves the file
  truncated. As a safety net, `format-yaml -I` writes the original content to `<file>.bak` just before the truncate
  and removes it after a successful write. If the tool refuses to run with `backup <file>.bak already exists`, a
  previous run crashed mid-write - inspect the backup and restore the target from it before rerunning
- In both variants the full formatted byte slice is buffered in memory before any file is opened for writing, so on a
  formatting error the tool fails early and never corrupts files

### Output is always UTF-8 with LF line endings

A file that arrived as UTF-16 or UTF-32 is transcoded to UTF-8 on read; CRLF endings become LF. If you need the original
encoding on disk, convert the formatted output back before writing.

### YAML directives (`%YAML 1.2`, `%TAG …`) are not preserved

They're rare and would need directive-aware doc splitting to round-trip cleanly. If your file needs them, don't run this
tool over it.

### Complex mapping keys (`? [a, b]: value`) are rejected at parse time by the underlying parser

The tool exits with a clear error; the file is not touched.

### Fuzz-only edge cases exist

Pathological YAML (embedded directive-looking tokens inside plain scalars, alias-like patterns not backed by a real
anchor) can produce output that goccy's own parser then rejects. Real config files don't hit this - it's a documented
limit, not a data-loss risk since nothing is written when parsing fails.

## WHY ANOTHER YAML FORMATTER

There are already several YAML formatters out there. `format-yaml` exists because none of them quite fit to my needs:

- **prettier** works well but drags a full Node.js runtime along with it, which is heavy for a one-file drop-in on a CI
  runner or a dev machine that otherwise has no Node. It is also intentionally opinionated to the point that important
  knobs (single vs. double quotes, block-scalar style, key sorting) either don't exist or aren't reachable from the CLI
- **yamlfix** hits the same weight problem in the Python direction (venv, pip install, transitive deps), and lacks
  features around block-scalar chomp selection and anchor-aware key sorting that this tool cares about
- Various YAML linters (yamllint et al.) only tell you what's wrong; they don't rewrite the file

`format-yaml` is a single binary you can drop into `$PATH` (or `mise use go:...`) with no runtime dependencies.

Yes, it's opinionated too - the difference is that I like my opinions :).

## AUTHOR

- Andrew Grechkin

## LICENSE

This project is licensed under the GNU General Public License Version 2 (GPLv2).
See the `LICENSE` file for details.
