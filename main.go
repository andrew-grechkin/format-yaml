package main

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"runtime/debug"

	"github.com/andrew-grechkin/format-yaml/internal/config"
	"github.com/andrew-grechkin/format-yaml/internal/format"
	"golang.org/x/term"
)

//go:embed help.txt
var gnuHelpText []byte

//go:embed README.md
var readmeContent []byte

func printReadme(out io.Writer) error {
	_, err := out.Write(readmeContent)
	return err
}

func printHelp(out io.Writer) error {
	_, err := out.Write(gnuHelpText)
	return err
}

func printVersion(out io.Writer) error {
	payload := []byte("{}")
	if info, ok := debug.ReadBuildInfo(); ok {
		payload, _ = json.MarshalIndent(info.Main, "", "  ")
	}
	_, err := fmt.Fprintln(out, string(payload))
	return err
}

func main() {
	err := run(os.Args, os.Stdin, os.Stdout)
	if err != nil && !errors.Is(err, format.ErrCheckDiff) {
		fmt.Fprintf(os.Stderr, "%v\n", err)
	}
	os.Exit(exitCode(err))
}

// Maps a run() error to the documented CLI exit contract:
//
//   - 0: success
//   - 1: --check or --diff found the input would be reformatted
//   - 2: parse or format error (malformed YAML, unsupported encoding)
//   - 3: I/O error (couldn't read, write, stat, or rename a file)
//
// I/O classification is done by type-checking the wrap chain for *fs.PathError or *os.LinkError - the two error types
// every std-lib filesystem call returns. goccy doesn't touch the filesystem, so a parse error can never masquerade as
// I/O by accident.
func exitCode(err error) int {
	switch {
	case err == nil:
		return 0
	case errors.Is(err, format.ErrCheckDiff):
		return 1
	case isIOError(err):
		return 3
	default:
		return 2
	}
}

func isIOError(err error) bool {
	var pathErr *fs.PathError
	if errors.As(err, &pathErr) {
		return true
	}
	var linkErr *os.LinkError
	return errors.As(err, &linkErr)
}

// Holds the parsed CLI shape: mode selection is separate (from env), argv only contributes flags and positional files.
type cliOptions struct {
	inplace  bool
	hardlink bool
	check    bool
	diff     bool
	files    []string
}

// Walks the argument list applying GNU conventions: a bare `--` starts a positional-only region so callers can pass
// filenames that would otherwise collide with flag names, and any unrecognised flag returns an error rather than
// silently ending up as a filename.
func parseArgs(args []string) (cliOptions, error) {
	var o cliOptions
	positional := false
	for _, a := range args {
		if positional {
			o.files = append(o.files, a)
			continue
		}
		switch a {
		case "--":
			positional = true
		case "-i", "--inplace":
			o.inplace = true
		case "-I":
			o.hardlink = true
		case "-c", "--check":
			o.check = true
		case "-d", "--diff":
			o.diff = true
		default:
			if len(a) > 1 && a[0] == '-' {
				return o, fmt.Errorf("unknown option %q (use -- to pass a filename starting with -)", a)
			}
			o.files = append(o.files, a)
		}
	}
	return o, nil
}

func run(args []string, stdin io.Reader, stdout io.Writer) error {
	if len(args) == 2 {
		switch args[1] {
		case "--version", "-v":
			return printVersion(stdout)
		case "--man", "-m":
			return printReadme(stdout)
		case "--help", "-h":
			return printHelp(stdout)
		}
	}
	opts, err := parseArgs(args[1:])
	if err != nil {
		return err
	}
	m, err := config.ParseMode(os.Getenv("FORMAT_YAML_MODE"))
	if err != nil {
		return err
	}
	lw, err := resolveLineWidth(os.Getenv("FORMAT_YAML_LINE_WIDTH"), stdout)
	if err != nil {
		return err
	}
	config.LineWidth = lw
	return dispatch(opts, m, stdin, stdout)
}

// Picks the active line width. An explicit FORMAT_YAML_LINE_WIDTH always wins. When unset, an interactive stdout
// narrows the default to (terminal width - 2) so wrapped output has a two-column safety gutter, but never widens past
// DefaultLineWidth. A non-TTY stdout (pipe, file, test buffer) keeps DefaultLineWidth so scripted output stays
// reproducible regardless of the invoking shell.
func resolveLineWidth(env string, out io.Writer) (int, error) {
	if env != "" {
		return config.ParseLineWidth(env)
	}

	f, ok := out.(*os.File)
	if !ok {
		return config.DefaultLineWidth, nil
	}

	fd := int(f.Fd())
	if !term.IsTerminal(fd) {
		return config.DefaultLineWidth, nil
	}

	w, _, err := term.GetSize(fd)
	if err != nil {
		return config.DefaultLineWidth, nil
	}

	return max(config.MinTTYLineWidth, min(w-2, config.DefaultLineWidth)), nil
}

// Routes to the right execution path based on the parsed flags. Kept separate so run stays under the
// cyclomatic-complexity budget as new modes are added.
func dispatch(opts cliOptions, m config.Mode, stdin io.Reader, stdout io.Writer) error {
	if err := validateFlags(opts); err != nil {
		return err
	}
	switch {
	case opts.inplace:
		return format.Inplace(opts.files, m)
	case opts.hardlink:
		return format.InplaceHardlink(opts.files, m)
	case opts.check && len(opts.files) == 0:
		return format.CheckReader(stdin, stdout, m)
	case opts.check:
		return format.CheckFiles(opts.files, stdout, m)
	case opts.diff && len(opts.files) == 0:
		return format.DiffReader(stdin, stdout, m)
	case opts.diff:
		return format.DiffFiles(opts.files, stdout, m)
	case len(opts.files) == 0:
		return format.Reader(stdin, stdout, m)
	default:
		return format.FilesToStdout(opts.files, stdout, m)
	}
}

// Rejects flag combinations that don't make sense before dispatching. Keeps the switch above readable.
func validateFlags(opts cliOptions) error {
	writeModes := 0
	if opts.inplace {
		writeModes++
	}
	if opts.hardlink {
		writeModes++
	}
	if opts.check {
		writeModes++
	}
	if opts.diff {
		writeModes++
	}
	if writeModes > 1 {
		return errors.New("--inplace, -I, --check, and --diff are mutually exclusive")
	}
	if opts.inplace && len(opts.files) == 0 {
		return errors.New("--inplace requires at least one file")
	}
	if opts.hardlink && len(opts.files) == 0 {
		return errors.New("-I requires at least one file")
	}
	return nil
}
