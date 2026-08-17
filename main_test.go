package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"testing"
	"unicode/utf16"

	goccyAst "github.com/goccy/go-yaml/ast"
	goccyParser "github.com/goccy/go-yaml/parser"

	"github.com/andrew-grechkin/update-yaml/pkg/style"

	"github.com/andrew-grechkin/format-yaml/internal/config"
	"github.com/andrew-grechkin/format-yaml/internal/encoding"
	"github.com/andrew-grechkin/format-yaml/internal/format"
)

// Pins down the CLI exit contract. Each classification path is exercised, including wrapped forms (the wrap chain is
// how the format package actually surfaces I/O errors from FilesToStdout / Inplace / etc.).
func TestExitCode(t *testing.T) {
	pathErr := &fs.PathError{Op: "open", Path: "x", Err: errors.New("no such file")}
	linkErr := &os.LinkError{Op: "rename", Old: "a", New: "b", Err: errors.New("cross-device")}
	cases := []struct {
		name string
		err  error
		want int
	}{
		{"nil is 0", nil, 0},
		{"errCheckDiff is 1", format.ErrCheckDiff, 1},
		{"wrapped errCheckDiff is 1", fmt.Errorf("outer: %w", format.ErrCheckDiff), 1},
		{"raw fs.PathError is 3", pathErr, 3},
		{"wrapped fs.PathError is 3", fmt.Errorf("read x: %w", pathErr), 3},
		{"os.LinkError is 3", linkErr, 3},
		{"wrapped os.LinkError is 3", fmt.Errorf("rename: %w", linkErr), 3},
		{"parse error is 2", errors.New("parse yaml: bad token"), 2},
		{"BOM decode error is 2", errors.New("UTF-16 input has odd byte length"), 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := exitCode(tc.err); got != tc.want {
				t.Errorf("exitCode(%v) = %d, want %d", tc.err, got, tc.want)
			}
		})
	}
}

func TestIsSafeToUnquote(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"hello", true},
		{"prod.example.com", true},
		{"https://example.com", true},
		{"it's", true},
		{"foo bar", true},
		{"foo:bar", true},

		{"true", false},
		{"false", false},
		{"null", false},
		{"~", false},
		{"1", false},
		{"1.0", false},
		{"0x1F", false},

		{"*foo", false},
		{"&foo", false},

		{"yes", false}, {"Yes", false}, {"YES", false},
		{"no", false}, {"No", false}, {"NO", false},
		{"on", false}, {"On", false}, {"ON", false},
		{"off", false}, {"Off", false}, {"OFF", false},
		{"y", false}, {"Y", false}, {"n", false}, {"N", false},

		{"1:30:00", false},
		{"1:2:3:4", false},
		{"9:59", false},
		{"99:0", false},
		{"80:80", true},
		{"80:99", true},
		{"9:60", true},
		{"0:0", true},

		{"0644", false}, {"010", false},

		{"192.168.1.1", true},
		{"10.0.0.1", true},
		{"127.0.0.1", true},
		{"255.255.255.255", true},

		{"::1", true},
		{"2001:db8::1", true},
		{"fe80::1", true},
		{"2001:0db8:0000:0000:0000:ff00:0042:8329", true},
		{"::ffff:192.0.2.1", true},
		{"1:2:3:4:5:6:7:8", false},
		{"2001:1:2:3:4:5:6:7", false},

		{"http://example.com", true},
		{"foo:bar", true},
		{"a:b:", false},

		{"", false},

		{" foo", false},
		{"foo ", false},

		{"line1\nline2", false},

		{"# not a comment inside string", false},

		{"---", false},
		{"...", false},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			if got := style.SafeToUnquote(tc.in); got != tc.want {
				t.Errorf("style.SafeToUnquote(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

// Exercises the strip pass through the full pipeline (format.Bytes) across every mode. Uses trailing whitespace in
// COMMENTS as the probe: goccy preserves comment trailing whitespace on re-emit (unlike plain-scalar values, which it
// strips on its own), so the presence/absence of the trailing spaces in the output genuinely depends on the strip pass.
// Block-scalar interior trailing whitespace must survive in every mode.
func TestFormatBytesStripsTrailingWhitespace(t *testing.T) {
	src := "a: 1 # note   \nb: 2\ndata: |\n  keep spaces   \n  another  \nafter: value # end   \n"
	for _, m := range []config.Mode{config.ModeMinimal, config.ModeStandard, config.ModeFull, config.ModePedantic} {
		out, err := format.Bytes([]byte(src), m)
		if err != nil {
			t.Fatalf("mode=%d format.Bytes: %v", m, err)
		}
		s := string(out)
		if strings.Contains(s, "# note   ") || strings.Contains(s, "# end   ") {
			t.Errorf("mode=%d: comment trailing whitespace survived outside blocks:\n%s", m, s)
		}
		if !strings.Contains(s, "  keep spaces   \n") || !strings.Contains(s, "  another  \n") {
			t.Errorf("mode=%d: block-scalar interior lost trailing whitespace:\n%s", m, s)
		}
	}
}

// Verifies BOM handling for every well-known encoding. UTF-8 BOM is stripped; UTF-16 and UTF-32 payloads (BE and LE)
// are transcoded to UTF-8 on the fly so the caller doesn't need to pre-convert. Output always emerges as UTF-8
// regardless of the input encoding.
func TestFormatBytesBOM(t *testing.T) {
	ascii := "a: 1\nb: 2\n"
	cases := []struct {
		name string
		src  []byte
	}{
		{"utf-8", append([]byte{0xEF, 0xBB, 0xBF}, []byte(ascii)...)},
		{"utf-16-be", append([]byte{0xFE, 0xFF}, encodeUTF16(ascii, true)...)},
		{"utf-16-le", append([]byte{0xFF, 0xFE}, encodeUTF16(ascii, false)...)},
		{"utf-32-be", append([]byte{0x00, 0x00, 0xFE, 0xFF}, encodeUTF32(ascii, true)...)},
		{"utf-32-le", append([]byte{0xFF, 0xFE, 0x00, 0x00}, encodeUTF32(ascii, false)...)},
		{"no bom", []byte(ascii)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, err := format.Bytes(tc.src, config.ModeStandard)
			if err != nil {
				t.Fatalf("format.Bytes: %v", err)
			}
			if bytes.HasPrefix(out, encoding.BOMUTF8) {
				t.Errorf("UTF-8 BOM leaked into output: %q", out)
			}
			if !bytes.Contains(out, []byte("a: 1")) {
				t.Errorf("expected `a: 1` in output, got %q", out)
			}
			if !bytes.Contains(out, []byte("b: 2")) {
				t.Errorf("expected `b: 2` in output, got %q", out)
			}
		})
	}
}

// Verifies transcoding errors on truncated UTF-16/UTF-32 payloads instead of silently producing garbage.
func TestFormatBytesBOMMalformed(t *testing.T) {
	cases := []struct {
		name string
		src  []byte
		want string
	}{
		{"utf-16-be odd length", append([]byte{0xFE, 0xFF}, 'a'), "odd byte length"},
		{"utf-32-le not aligned", append([]byte{0xFF, 0xFE, 0x00, 0x00}, 'a', 'b'), "not a multiple of 4"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := format.Bytes(tc.src, config.ModeStandard)
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("err = %v, want substring %q", err, tc.want)
			}
		})
	}
}

func encodeUTF16(s string, bigEndian bool) []byte {
	units := utf16.Encode([]rune(s))
	out := make([]byte, 2*len(units))
	for i, u := range units {
		if bigEndian {
			out[2*i], out[2*i+1] = byte(u>>8), byte(u)
		} else {
			out[2*i], out[2*i+1] = byte(u), byte(u>>8)
		}
	}
	return out
}

func encodeUTF32(s string, bigEndian bool) []byte {
	runes := []rune(s)
	out := make([]byte, 4*len(runes))
	for i, r := range runes {
		u := uint32(r)
		if bigEndian {
			out[4*i] = byte(u >> 24)
			out[4*i+1] = byte(u >> 16)
			out[4*i+2] = byte(u >> 8)
			out[4*i+3] = byte(u)
		} else {
			out[4*i] = byte(u)
			out[4*i+1] = byte(u >> 8)
			out[4*i+2] = byte(u >> 16)
			out[4*i+3] = byte(u >> 24)
		}
	}
	return out
}

// Verifies Windows line endings normalize to LF without breaking parsing or idempotence.
func TestFormatBytesCRLF(t *testing.T) {
	out, err := format.Bytes([]byte("a: 1\r\nb: 2\r\n"), config.ModeStandard)
	if err != nil {
		t.Fatalf("format.Bytes: %v", err)
	}
	if bytes.Contains(out, []byte("\r")) {
		t.Errorf("output should not contain \\r, got %q", out)
	}
	out2, err := format.Bytes(out, config.ModeStandard)
	if err != nil {
		t.Fatalf("second pass: %v", err)
	}
	if !bytes.Equal(out, out2) {
		t.Errorf("CRLF round-trip not idempotent\nfirst: %q\nsecond: %q", out, out2)
	}
}

// Covers YAML shapes that no fixture exercises directly: tags, root sequences, aliases-to-scalar, flow-style leaves.
func TestFormatBytesYAMLShapes(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want string
	}{
		{"tag preserved", "x: !!str 42\n", "!!str"},
		{"custom tag preserved", "z: !MyTag some_value\n", "!MyTag"},
		{"scalar anchor and alias", "x: &a hello\ny: *a\n", "*a"},
		{"root sequence", "- one\n- two\n", "- one"},
		{"empty flow map", "empty: {}\n", "{}"},
		{"empty flow seq", "empty: []\n", "[]"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, err := format.Bytes([]byte(tc.src), config.ModeStandard)
			if err != nil {
				t.Fatalf("format.Bytes: %v", err)
			}
			if !bytes.Contains(out, []byte(tc.want)) {
				t.Errorf("output missing %q\ngot:\n%s", tc.want, out)
			}
			out2, err := format.Bytes(out, config.ModeStandard)
			if err != nil {
				t.Fatalf("second pass: %v", err)
			}
			if !bytes.Equal(out, out2) {
				t.Errorf("not idempotent\nfirst: %q\nsecond: %q", out, out2)
			}
		})
	}
}

// The reason -I exists at all. If this ever regresses, hardlinks would silently detach when a user picked -I precisely
// to avoid that.
func TestFormatInplaceHardlinkPreservesInode(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.yaml")
	b := filepath.Join(dir, "b.yaml")
	if err := os.WriteFile(a, []byte("k: 1\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(a, b); err != nil {
		t.Fatalf("hardlink: %v", err)
	}
	beforeIno := inode(t, a)
	if err := format.InplaceHardlink([]string{a}, config.ModeStandard); err != nil {
		t.Fatalf("format.InplaceHardlink: %v", err)
	}
	if inode(t, a) != beforeIno {
		t.Fatalf("inode changed: was %d now %d", beforeIno, inode(t, a))
	}
	bBytes, err := os.ReadFile(b)
	if err != nil {
		t.Fatal(err)
	}
	aBytes, err := os.ReadFile(a)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(aBytes, bBytes) {
		t.Errorf("hardlink diverged after -I\n  a: %q\n  b: %q", aBytes, bBytes)
	}
}

// Verifies -I refuses to run when a stale <path>.bak already exists - protects against destroying crash-recovery data
// from a prior aborted run.
func TestFormatInplaceHardlinkBackupRefused(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "cfg.yaml")
	if err := os.WriteFile(target, []byte("k: 1\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target+".bak", []byte("stale\n"), 0644); err != nil {
		t.Fatal(err)
	}
	err := format.InplaceHardlink([]string{target}, config.ModeStandard)
	if err == nil {
		t.Fatal("expected refusal error, got nil")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("expected 'already exists' in error, got %v", err)
	}
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "k: 1\n" {
		t.Errorf("target was modified despite refusal: %q", got)
	}
}

// Verifies the .bak file appears during a successful -I run and is cleaned up afterward.
func TestFormatInplaceHardlinkCreatesAndRemovesBackup(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "cfg.yaml")
	src := "b: 2\na: 1\n"
	if err := os.WriteFile(target, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}
	if err := format.InplaceHardlink([]string{target}, config.ModeFull); err != nil {
		t.Fatalf("format.InplaceHardlink: %v", err)
	}
	if _, err := os.Stat(target + ".bak"); !os.IsNotExist(err) {
		t.Errorf(".bak should not exist after success, stat err = %v", err)
	}
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(got, []byte("a: 1")) || !bytes.Contains(got, []byte("b: 2")) {
		t.Errorf("target content unexpected: %q", got)
	}
}

// Counterpart to the -I hardlink test: pins down that -i does the tmp+rename dance, which intentionally creates a new
// inode. Hardlinks pointing at the old inode diverge - if a user needs them preserved, they should reach for -I, not
// -i.
func TestFormatInplaceAtomicChangesInode(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.yaml")
	b := filepath.Join(dir, "b.yaml")
	original := "k: 1\n"
	if err := os.WriteFile(a, []byte(original), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(a, b); err != nil {
		t.Fatalf("hardlink: %v", err)
	}
	beforeIno := inode(t, a)
	if err := format.Inplace([]string{a}, config.ModeStandard); err != nil {
		t.Fatalf("format.Inplace: %v", err)
	}
	if inode(t, a) == beforeIno {
		t.Fatalf("expected inode to change on -i, still %d", beforeIno)
	}
	bBytes, err := os.ReadFile(b)
	if err != nil {
		t.Fatal(err)
	}
	if string(bBytes) != original {
		t.Errorf("hardlink at old inode should still see original content, got %q", bBytes)
	}
}

// Verifies that when a later file in the batch fails to format, no earlier file has been touched - the two-phase
// (format-all-first, write-all-second) structure means disk writes only start once every input is known to be valid.
func TestFormatInplaceAtomicAllOrNothing(t *testing.T) {
	dir := t.TempDir()
	good1 := filepath.Join(dir, "a.yaml")
	bad := filepath.Join(dir, "bad.yaml")
	good2 := filepath.Join(dir, "c.yaml")
	good1Src := "a: 1\n"
	good2Src := "c: 3\n"
	if err := os.WriteFile(good1, []byte(good1Src), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bad, []byte{0xFE, 0xFF, 'x'}, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(good2, []byte(good2Src), 0644); err != nil {
		t.Fatal(err)
	}

	err := format.Inplace([]string{good1, bad, good2}, config.ModeStandard)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "bad.yaml") {
		t.Errorf("error should identify the failing file: %v", err)
	}
	if got, _ := os.ReadFile(good1); string(got) != good1Src {
		t.Errorf("good1 was modified despite atomic guarantee: %q", got)
	}
	if got, _ := os.ReadFile(good2); string(got) != good2Src {
		t.Errorf("good2 was modified despite atomic guarantee: %q", got)
	}
}

// Verifies the tmp+rename path carries over the original file mode rather than leaving the target at CreateTemp's 0600.
func TestFormatInplaceAtomicPreservesPerm(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "cfg.yaml")
	if err := os.WriteFile(target, []byte("b: 2\na: 1\n"), 0640); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(target, 0640); err != nil {
		t.Fatal(err)
	}
	if err := format.Inplace([]string{target}, config.ModeFull); err != nil {
		t.Fatalf("format.Inplace: %v", err)
	}
	st, err := os.Stat(target)
	if err != nil {
		t.Fatal(err)
	}
	if perm := st.Mode().Perm(); perm != 0640 {
		t.Errorf("permissions changed: was 0640, now %#o", perm)
	}
}

func inode(t *testing.T, path string) uint64 {
	t.Helper()
	st, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	sys, ok := st.Sys().(*syscall.Stat_t)
	if !ok {
		t.Skip("no syscall.Stat_t available on this platform")
	}
	return sys.Ino
}

// Covers the non-TTY branches of resolveLineWidth: an explicit env value always wins, an empty env with a writer that
// isn't a terminal keeps the compiled-in default, and a malformed env surfaces the parser error. The TTY branch itself
// needs a real pty to exercise and is left to manual verification.
func TestResolveLineWidth(t *testing.T) {
	nonTTY, err := os.CreateTemp(t.TempDir(), "lw-*")
	if err != nil {
		t.Fatal(err)
	}
	defer nonTTY.Close()
	cases := []struct {
		name    string
		env     string
		out     io.Writer
		want    int
		wantErr bool
	}{
		{"empty env, buffer writer", "", &bytes.Buffer{}, config.DefaultLineWidth, false},
		{"empty env, discard writer", "", io.Discard, config.DefaultLineWidth, false},
		{"empty env, non-tty file", "", nonTTY, config.DefaultLineWidth, false},
		{"explicit env overrides", "80", &bytes.Buffer{}, 80, false},
		{"explicit env wider than default", "200", &bytes.Buffer{}, 200, false},
		{"invalid env is an error", "abc", &bytes.Buffer{}, 0, true},
		{"zero env is rejected", "0", &bytes.Buffer{}, 0, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := resolveLineWidth(tc.env, tc.out)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil (returned %d)", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("resolveLineWidth(%q) = %d, want %d", tc.env, got, tc.want)
			}
		})
	}
}

// Exercises --check via run(): dirty input returns errCheckDiff, clean input returns nil.
func TestRunCheck(t *testing.T) {
	dirty := "a: 1\nb: 2\n"
	clean, err := format.Bytes([]byte(dirty), config.ModeStandard)
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name   string
		src    string
		wantOK bool
	}{
		{"dirty", dirty, false},
		{"clean", string(clean), true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer
			err := run([]string{"format-yaml", "--check"}, bytes.NewReader([]byte(tc.src)), &out)
			if tc.wantOK && err != nil {
				t.Fatalf("expected nil, got %v", err)
			}
			if !tc.wantOK && err != format.ErrCheckDiff {
				t.Fatalf("expected format.ErrCheckDiff, got %v", err)
			}
		})
	}
}

// Verifies --diff prints a unified diff and reports the non-zero exit via errCheckDiff when input isn't already
// formatted. A clean input emits no diff and returns nil.
func TestRunDiff(t *testing.T) {
	dirty := "a: 1\nb: 2\n"
	var out bytes.Buffer
	err := run([]string{"format-yaml", "--diff"}, bytes.NewReader([]byte(dirty)), &out)
	if err != format.ErrCheckDiff {
		t.Fatalf("expected format.ErrCheckDiff, got %v", err)
	}
	got := out.String()
	for _, want := range []string{"--- <stdin> (original)", "+++ <stdin> (formatted)", "+---"} {
		if !strings.Contains(got, want) {
			t.Errorf("diff missing %q\n---\n%s", want, got)
		}
	}
	clean, _ := format.Bytes([]byte(dirty), config.ModeStandard)
	var cleanOut bytes.Buffer
	if err := run([]string{"format-yaml", "--diff"}, bytes.NewReader(clean), &cleanOut); err != nil {
		t.Fatalf("clean input expected nil, got %v", err)
	}
	if cleanOut.Len() != 0 {
		t.Errorf("clean input should emit no diff, got %q", cleanOut.String())
	}
}

// Pins down the flag-parser corner cases: `--` starts a positional-only region so filenames that collide with flag
// spellings can still be passed, and unknown flags produce a clear error rather than silently becoming filenames.
func TestParseArgs(t *testing.T) {
	cases := []struct {
		name      string
		args      []string
		wantFiles []string
		wantCheck bool
		wantDiff  bool
		wantErr   bool
	}{
		{"no args", nil, nil, false, false, false},
		{"single file", []string{"a.yaml"}, []string{"a.yaml"}, false, false, false},
		{"flag then file", []string{"--check", "a.yaml"}, []string{"a.yaml"}, true, false, false},
		{"double-dash separator", []string{"--check", "--", "-d.yaml", "--foo.yaml"}, []string{"-d.yaml", "--foo.yaml"}, true, false, false},
		{"double-dash only", []string{"--", "-d"}, []string{"-d"}, false, false, false},
		{"unknown flag", []string{"--unknown"}, nil, false, false, true},
		{"typo of known flag", []string{"--chek"}, nil, false, false, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseArgs(tc.args)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, wantErr = %v", err, tc.wantErr)
			}
			if tc.wantErr {
				return
			}
			if !reflect.DeepEqual(got.files, tc.wantFiles) {
				t.Errorf("files = %q, want %q", got.files, tc.wantFiles)
			}
			if got.check != tc.wantCheck {
				t.Errorf("check = %v, want %v", got.check, tc.wantCheck)
			}
			if got.diff != tc.wantDiff {
				t.Errorf("diff = %v, want %v", got.diff, tc.wantDiff)
			}
		})
	}
}

// Verifies validateFlags rejects the combinations that don't make sense.
func TestRunFlagConflicts(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{"check + inplace", []string{"format-yaml", "--check", "--inplace", "f.yaml"}},
		{"diff + inplace", []string{"format-yaml", "--diff", "--inplace", "f.yaml"}},
		{"check + diff", []string{"format-yaml", "--check", "--diff"}},
		{"inplace no files", []string{"format-yaml", "--inplace"}},
		{"inplace + hardlink", []string{"format-yaml", "--inplace", "-I", "f.yaml"}},
		{"hardlink + check", []string{"format-yaml", "-I", "--check", "f.yaml"}},
		{"hardlink + diff", []string{"format-yaml", "-I", "--diff", "f.yaml"}},
		{"hardlink no files", []string{"format-yaml", "-I"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := run(tc.args, bytes.NewReader(nil), io.Discard)
			if err == nil {
				t.Errorf("expected error, got nil")
			}
		})
	}
}

// Drives run() end-to-end with each inplace flag so a regression in dispatch (e.g., -i and -I swapped, or one variant
// wired to the wrong implementation) fails here. The internal format.Inplace / format.InplaceHardlink tests bypass
// parseArgs and dispatch, so they wouldn't catch a routing bug on their own.
func TestRunInplaceRouting(t *testing.T) {
	cases := []struct {
		name             string
		flag             string
		wantInodeChanged bool
	}{
		{"--inplace changes inode", "--inplace", true},
		{"-i changes inode", "-i", true},
		{"-I preserves inode", "-I", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			target := filepath.Join(dir, "cfg.yaml")
			if err := os.WriteFile(target, []byte("k: 1\n"), 0644); err != nil {
				t.Fatal(err)
			}
			before := inode(t, target)
			err := run([]string{"format-yaml", tc.flag, target}, bytes.NewReader(nil), io.Discard)
			if err != nil {
				t.Fatalf("run: %v", err)
			}
			after := inode(t, target)
			if tc.wantInodeChanged && after == before {
				t.Errorf("%s: expected inode to change, still %d", tc.flag, before)
			}
			if !tc.wantInodeChanged && after != before {
				t.Errorf("%s: expected inode preserved, was %d now %d", tc.flag, before, after)
			}
		})
	}
}

// Pins down what happens when the argument is a symlink under both in-place variants. For -i, the tmp+rename dance
// would replace the symlink entry itself with the tmp file unless writeAtomic follows the link first. For -I,
// os.WriteFile follows the symlink and writes through, so it should just work.
//
// Both flags must leave the symlink intact and update the pointed-to file.
func TestFormatInplaceSymlinkBehavior(t *testing.T) {
	cases := []struct{ name, flag string }{
		{"--inplace follows symlink", "-i"},
		{"-I follows symlink", "-I"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			target := filepath.Join(dir, "real.yaml")
			link := filepath.Join(dir, "link.yaml")
			if err := os.WriteFile(target, []byte("b: 2\na: 1\n"), 0644); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(target, link); err != nil {
				t.Fatalf("symlink: %v", err)
			}
			if err := run([]string{"format-yaml", tc.flag, link}, bytes.NewReader(nil), io.Discard); err != nil {
				t.Fatalf("run: %v", err)
			}
			info, err := os.Lstat(link)
			if err != nil {
				t.Fatal(err)
			}
			if info.Mode()&os.ModeSymlink == 0 {
				t.Fatalf("%s: symlink was replaced with a regular file", tc.flag)
			}
			pointsTo, err := os.Readlink(link)
			if err != nil {
				t.Fatal(err)
			}
			if pointsTo != target {
				t.Errorf("%s: symlink now points to %q, want %q", tc.flag, pointsTo, target)
			}
			got, err := os.ReadFile(target)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.HasPrefix(got, []byte("---\n")) {
				t.Errorf("%s: pointed-to file wasn't formatted, got %q", tc.flag, got)
			}
		})
	}
}

// Pins down behavior on tiny/degenerate inputs that don't fit the regular fixture pattern (no key-value pairs to
// compare against). It only asserts that the tool doesn't panic and that output is idempotent under the same mode.
func TestFormatBytesEdges(t *testing.T) {
	cases := []struct {
		name string
		src  string
	}{
		{"empty", ""},
		{"whitespace only", "   \n\t\n"},
		{"single top-level key", "a: 1\n"},
		{"comment-only file", "# lone comment\n"},
		{"multiple comments no body", "# one\n# two\n# three\n"},
		{"null body via tilde", "a: ~\n"},
		{"empty mapping value", "a:\n"},
		{"deep nesting", "a:\n  b:\n    c:\n      d: leaf\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for m := config.ModeMinimal; m <= config.ModePedantic; m++ {
				out1, err := format.Bytes([]byte(tc.src), m)
				if err != nil {
					t.Fatalf("mode=%d first pass err=%v", m, err)
				}
				out2, err := format.Bytes(out1, m)
				if err != nil {
					t.Fatalf("mode=%d second pass err=%v\nout1=%q", m, err, out1)
				}
				if !bytes.Equal(out1, out2) {
					t.Errorf("mode=%d not idempotent\nout1=%q\nout2=%q", m, out1, out2)
				}
			}
		})
	}
}

// Pins the plain-scalar-to-`>-` conversion. Value preservation is the hard invariant: every case here re-parses to a
// value byte-identical to the source. The pass is deliberately conservative - the "must stay plain" cases prove the
// guard rails fire even when the line is well over the configured line width.
func TestFoldLongScalars(t *testing.T) {
	longWords := strings.Repeat("word ", 40) + "end"

	cases := []struct {
		name      string
		src       string
		wantVal   string
		folded    bool
		passWraps bool
	}{
		{
			name:      "basic long plain scalar folds",
			src:       "text: " + longWords + "\n",
			wantVal:   longWords,
			folded:    true,
			passWraps: true,
		},
		{
			name:      "interior double-space run survives fold",
			src:       "text: " + strings.Repeat("word ", 15) + "hello  world" + strings.Repeat(" word", 20) + "\n",
			wantVal:   strings.Repeat("word ", 15) + "hello  world" + strings.Repeat(" word", 20),
			folded:    true,
			passWraps: true,
		},
		{
			name:      "leading dash content folds (would break in plain)",
			src:       "text: \"- " + longWords + "\"\n",
			wantVal:   "- " + longWords,
			folded:    true,
			passWraps: true,
		},
		{
			name:      "interior colon folds",
			src:       "text: \"key: " + longWords + "\"\n",
			wantVal:   "key: " + longWords,
			folded:    true,
			passWraps: true,
		},
		{
			name:    "leading whitespace stays plain (guard rail)",
			src:     "text: \"   " + longWords + "\"\n",
			wantVal: "   " + longWords,
			folded:  false,
		},
		{
			name:    "trailing whitespace stays plain (guard rail)",
			src:     "text: \"" + longWords + "   \"\n",
			wantVal: longWords + "   ",
			folded:  false,
		},
		{
			name:    "no space characters stays plain (no wrap position)",
			src:     "text: " + strings.Repeat("a", 200) + "\n",
			wantVal: strings.Repeat("a", 200),
			folded:  false,
		},
		{
			name:    "short line stays plain (no overflow)",
			src:     "text: hello world\n",
			wantVal: "hello world",
			folded:  false,
		},
		{
			name:      "source already `>-` stays as-is (not a StringNode)",
			src:       "text: >-\n  " + longWords + "\n",
			wantVal:   longWords,
			folded:    true,
			passWraps: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, err := format.Bytes([]byte(tc.src), config.ModeFull)
			if err != nil {
				t.Fatalf("format.Bytes: %v", err)
			}
			gotVal := parseFirstScalarValue(t, out)
			if gotVal != tc.wantVal {
				t.Errorf("value drift\ngot:  %q\nwant: %q\nfull output:\n%s", gotVal, tc.wantVal, out)
			}
			gotFolded := bytes.Contains(out, []byte("text: >-\n"))
			if gotFolded != tc.folded {
				t.Errorf("folded=%v want=%v\noutput:\n%s", gotFolded, tc.folded, out)
			}
			out2, err := format.Bytes(out, config.ModeFull)
			if err != nil {
				t.Fatalf("second pass err: %v", err)
			}
			if !bytes.Equal(out, out2) {
				t.Errorf("not idempotent\nfirst:\n%s\nsecond:\n%s", out, out2)
			}
			if tc.passWraps {
				for line := range bytes.SplitSeq(out, []byte("\n")) {
					if style.DisplayWidth(string(line)) > config.LineWidth {
						t.Errorf("wrap line exceeds line width: %q (width %d > %d)", line, style.DisplayWidth(string(line)), config.LineWidth)
					}
				}
			}
		})
	}
}

// Reads the parsed value of the first top-level mapping entry. Handles plain / quoted / block-scalar styles - which is
// exactly the set of representations foldLongScalars might produce.
func parseFirstScalarValue(t *testing.T, src []byte) string {
	t.Helper()
	file, err := goccyParser.ParseBytes(src, 0)
	if err != nil {
		t.Fatalf("re-parse: %v", err)
	}
	if len(file.Docs) == 0 {
		t.Fatal("no docs in output")
	}
	mn, ok := file.Docs[0].Body.(*goccyAst.MappingNode)
	if !ok {
		t.Fatalf("first doc body is %T, want *ast.MappingNode", file.Docs[0].Body)
	}
	if len(mn.Values) == 0 {
		t.Fatal("empty mapping")
	}
	v := mn.Values[0].Value
	switch node := v.(type) {
	case *goccyAst.StringNode:
		return node.Value
	case *goccyAst.LiteralNode:
		if node.Value == nil {
			return ""
		}
		return node.Value.Value
	default:
		t.Fatalf("unexpected value node type %T", v)
		return ""
	}
}

// Inline flow at mode full: the mirror of unflowLongFlows. Each case asserts an exact output rather than a substring so
// an off-by-one in the width check or a missed disqualifier surfaces as a precise diff.
func TestFlowShortBlocks(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want string
	}{
		{
			name: "plain-safe short sequence folds",
			src:  "letters:\n  - a\n  - b\n  - c\n",
			want: "---\nletters: [a, b, c]\n",
		},
		{
			name: "undroppable-quote item blocks fold (mirror rule)",
			src:  "mixed:\n  - a\n  - \"42\"\n  - b\n",
			want: "---\nmixed:\n  - a\n  - '42'\n  - b\n",
		},
		{
			name: "folded line over the line width stays block",
			src: "data:\n  - " + strings.Repeat("a", config.DefaultLineWidth/3) + "\n  - " +
				strings.Repeat("b", config.DefaultLineWidth/3) + "\n  - " +
				strings.Repeat("c", config.DefaultLineWidth/3) + "\n",
			want: "---\ndata:\n  - " + strings.Repeat("a", config.DefaultLineWidth/3) + "\n  - " +
				strings.Repeat("b", config.DefaultLineWidth/3) + "\n  - " +
				strings.Repeat("c", config.DefaultLineWidth/3) + "\n",
		},
		{
			name: "anchor on item blocks fold",
			src:  "data:\n  - &first alpha\n  - beta\n",
			want: "---\ndata:\n  - &first alpha\n  - beta\n",
		},
		{
			name: "nested collection item blocks fold",
			src:  "data:\n  - - inner\n  - - other\n",
			want: "---\ndata:\n  - - inner\n  - - other\n",
		},
		{
			name: "already flow stays flow",
			src:  "letters: [a, b, c]\n",
			want: "---\nletters: [a, b, c]\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, err := format.Bytes([]byte(tc.src), config.ModeFull)
			if err != nil {
				t.Fatalf("format.Bytes: %v", err)
			}
			if string(out) != tc.want {
				t.Errorf("got:\n%s\nwant:\n%s", out, tc.want)
			}
			out2, err := format.Bytes(out, config.ModeFull)
			if err != nil {
				t.Fatalf("second pass: %v", err)
			}
			if !bytes.Equal(out, out2) {
				t.Errorf("not idempotent\nfirst:\n%s\nsecond:\n%s", out, out2)
			}
		})
	}
}

// Exercises both unflow triggers at mode full: a line whose display width crosses lineWidth, and any collection with a
// scalar item that couldn't be represented plain (an undroppable quote). The two triggers are OR'd - either fires the
// unflow.
func TestUnflowLongFlows(t *testing.T) {
	wide1 := strings.Repeat("a", config.DefaultLineWidth)
	wide2 := strings.Repeat("b", config.DefaultLineWidth)
	cases := []struct {
		name string
		src  string
		want string
	}{
		{
			name: "wide line unflows to block",
			src:  "keys: [" + wide1 + ", " + wide2 + "]\n",
			want: "---\nkeys:\n  - " + wide1 + "\n  - " + wide2 + "\n",
		},
		{
			name: "undroppable quote unflows even when line fits",
			src:  "ports: [80, \"22\", 443]\n",
			want: "---\nports:\n  - 80\n  - '22'\n  - 443\n",
		},
		{
			name: "short plain flow stays flow",
			src:  "letters: [a, b, c]\n",
			want: "---\nletters: [a, b, c]\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, err := format.Bytes([]byte(tc.src), config.ModeFull)
			if err != nil {
				t.Fatalf("format.Bytes: %v", err)
			}
			if string(out) != tc.want {
				t.Errorf("got:\n%s\nwant:\n%s", out, tc.want)
			}
			out2, err := format.Bytes(out, config.ModeFull)
			if err != nil {
				t.Fatalf("second pass: %v", err)
			}
			if !bytes.Equal(out, out2) {
				t.Errorf("not idempotent\nfirst:\n%s\nsecond:\n%s", out, out2)
			}
		})
	}
}

// Pins down the "block scalar body indent = parent col + 2" rule that reindentBlockScalars enforces. The interesting
// cases are the ones where the body was NOT at the target indent before the pass: JSON-flow sources that spawn a fresh
// LiteralNode via blockScalarizeMultiline, and source-level scalars whose parent key gets shifted by normalizeIndent.
// Folded blocks (`>`) are outside the scope by design - the pass leaves them alone even when their indent looks wrong.
func TestReindentBlockScalars(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want string
	}{
		{
			name: "JSON flow with newline scalar produces valid |- body",
			src:  `[{"col": "line1\nline2"}]` + "\n",
			want: "---\n- col: |-\n    line1\n    line2\n",
		},
		{
			name: "source |- already at correct indent stays correct",
			src:  "- key: |-\n    body1\n    body2\n",
			want: "---\n- key: |-\n    body1\n    body2\n",
		},
		{
			name: "top-level |- body sits at col 3 (parent col 1 + 2)",
			src:  "note: \"line1\\nline2\"\n",
			want: "---\nnote: |-\n  line1\n  line2\n",
		},
		{
			name: "|+ trailing newlines preserved after reindent",
			src:  "text: \"line1\\nline2\\n\\n\"\n",
			want: "---\ntext: |+\n  line1\n  line2\n\n...\n",
		},
		{
			name: "folded > block is left alone (out of scope by design)",
			src:  "note: >\n  folded content across\n  multiple lines\n",
			want: "---\nnote: >\n  folded content across\n  multiple lines\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, err := format.Bytes([]byte(tc.src), config.ModeFull)
			if err != nil {
				t.Fatalf("format.Bytes: %v", err)
			}
			if string(out) != tc.want {
				t.Errorf("got:\n%s\nwant:\n%s", out, tc.want)
			}
			out2, err := format.Bytes(out, config.ModeFull)
			if err != nil {
				t.Fatalf("second pass (invalid YAML from first?): %v\nfirst:\n%s", err, out)
			}
			if !bytes.Equal(out, out2) {
				t.Errorf("not idempotent\nfirst:\n%s\nsecond:\n%s", out, out2)
			}
		})
	}
}

// Pins down the buffered guarantee: if file N fails to format, files 1..N-1 must not have been written to the output
// stream. Otherwise a shell pipeline like `format-yaml a b c > out` could leave `out` half-written when b fails, and
// the user would need to know that on their own.
func TestFormatFilesToStdoutAllOrNothing(t *testing.T) {
	dir := t.TempDir()
	good1 := filepath.Join(dir, "a.yaml")
	bad := filepath.Join(dir, "bad.yaml")
	good2 := filepath.Join(dir, "c.yaml")
	if err := os.WriteFile(good1, []byte("a: 1\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bad, []byte{0xFE, 0xFF, 'x'}, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(good2, []byte("c: 3\n"), 0644); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	err := format.FilesToStdout([]string{good1, bad, good2}, &out, config.ModeStandard)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "bad.yaml") {
		t.Errorf("error should identify the failing file: %v", err)
	}
	if out.Len() != 0 {
		t.Errorf("expected zero bytes written on failure; got %d bytes: %q", out.Len(), out.Bytes())
	}
}
