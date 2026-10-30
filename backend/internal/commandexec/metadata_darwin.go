package commandexec

import (
	"io/fs"
	"path/filepath"
	"syscall"

	"golang.org/x/sys/unix"
)

// The project transaction represents files, directories, modes and symbolic
// links. Reject metadata it cannot commit faithfully; this is not OS isolation.
func ValidateCandidateMetadata(root string) error {
	return filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if path == root && !info.IsDir() {
			return commandError(CodeNotAllowed, "Command replaced its workspace root.")
		}
		if info.Mode()&(fs.ModeSetuid|fs.ModeSetgid|fs.ModeSticky) != 0 {
			return commandError(CodeNotAllowed, "Special permission bits cannot be represented by project transactions: "+path)
		}
		if stat, ok := info.Sys().(*syscall.Stat_t); ok && info.Mode().IsRegular() && stat.Nlink > 1 {
			return commandError(CodeNotAllowed, "Hard links cannot be represented by project transactions: "+path)
		}
		size, err := unix.Llistxattr(path, nil)
		if err != nil {
			return err
		}
		if size > 0 {
			return commandError(CodeNotAllowed, "Extended attributes cannot be represented by project transactions: "+path)
		}
		return nil
	})
}
