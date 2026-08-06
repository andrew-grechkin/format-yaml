package format

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/andrew-grechkin/format-yaml/internal/config"
)

// Bound to -i / --inplace. Rewrites each file via the classic temp-file-plus-rename dance: format all files into memory
// first, then for every file write a temp sibling and atomically rename it over the target. rename(2) is atomic on
// POSIX so a crash never leaves the target half-written. Side effect: the file's inode changes, so any hardlinks
// pointing at the original inode continue to see the pre-format content - use -I when that matters.
//
// The two-phase structure (format-all, then write-all) means a parse or format error on any file aborts the whole run
// before any file is touched, matching the all-or-nothing guarantee used by stdout mode. Once the second phase starts,
// a write failure on file N leaves files 1..N-1 already updated - unavoidable without cross-file transactions.
func Inplace(paths []string, m config.Mode) error {
	rendered := make([][]byte, len(paths))
	for i, p := range paths {
		src, err := os.ReadFile(p)
		if err != nil {
			return fmt.Errorf("read %s: %w", p, err)
		}
		formatted, err := Bytes(src, m)
		if err != nil {
			return fmt.Errorf("%s: %w", p, err)
		}
		rendered[i] = formatted
	}
	for i, p := range paths {
		if err := writeAtomic(p, rendered[i]); err != nil {
			return err
		}
	}
	return nil
}

// Writes data to a temp sibling of target and renames it over target. The temp lives in the same directory so rename(2)
// stays within one filesystem (cross-filesystem rename returns EXDEV). Target permissions are preserved by chmod'ing
// the temp to match before the rename; without it, the target would inherit CreateTemp's 0600.
//
// If target is a symlink, we resolve it first and operate on the pointed-to file. rename(2) applied directly to a
// symlink entry replaces the symlink with the tmp file, which is almost never what the caller of -i wants - they'd
// expect the symlink to keep pointing at the (now-updated) real file, matching how sed -i and friends behave.
func writeAtomic(target string, data []byte) error {
	resolved, err := filepath.EvalSymlinks(target)
	if err != nil {
		return fmt.Errorf("resolve %s: %w", target, err)
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return fmt.Errorf("stat %s: %w", resolved, err)
	}
	perm := info.Mode().Perm()
	dir := filepath.Dir(resolved)
	base := filepath.Base(resolved)
	tmp, err := os.CreateTemp(dir, "."+base+".*.tmp")
	if err != nil {
		return fmt.Errorf("create temp for %s: %w", target, err)
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("write temp for %s: %w", target, err)
	}
	if err := tmp.Chmod(perm); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("chmod temp for %s: %w", target, err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("close temp for %s: %w", target, err)
	}
	if err := os.Rename(tmpName, resolved); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("rename temp to %s: %w", target, err)
	}
	return nil
}

// Bound to -I / --inplace-hardlink. Writes each file by truncating and rewriting it in place via os.WriteFile. The
// file's inode is preserved, which keeps any hardlinks pointing at the same content - the reason we avoid the
// temp-file-plus-rename dance that Inplace uses.
//
// A `<path>.bak` copy is written just before the truncate and removed on success, so a crash between truncate and
// completed write leaves the user with recoverable content. If `<path>.bak` already exists at the start of a run, we
// refuse to proceed: the most likely cause is a prior crashed run, and blindly overwriting the backup would destroy the
// very recovery data it was created to preserve.
//
// The buffered pipeline still guarantees the complete formatted bytes exist in memory before either file is opened for
// writing, so a formatting error never touches disk.
func InplaceHardlink(paths []string, m config.Mode) error {
	for _, p := range paths {
		src, err := os.ReadFile(p)
		if err != nil {
			return fmt.Errorf("read %s: %w", p, err)
		}
		formatted, err := Bytes(src, m)
		if err != nil {
			return fmt.Errorf("%s: %w", p, err)
		}
		backup := p + ".bak"
		if _, err := os.Stat(backup); err == nil {
			return fmt.Errorf("backup %s already exists; inspect it before rerunning", backup)
		} else if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("stat %s: %w", backup, err)
		}
		if err := os.WriteFile(backup, src, 0644); err != nil {
			return fmt.Errorf("write backup %s: %w", backup, err)
		}
		if err := os.WriteFile(p, formatted, 0644); err != nil {
			return fmt.Errorf("write %s (backup preserved at %s): %w", p, backup, err)
		}
		if err := os.Remove(backup); err != nil {
			return fmt.Errorf("remove backup %s: %w", backup, err)
		}
	}
	return nil
}
