//go:build !darwin

package commandexec

import "os"

func applySymlinkMode(path string, mode uint32) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if uint32(info.Mode().Perm()) != mode {
		return commandError(CodeNotAvailable, "Symbolic-link permission changes are unsupported on this platform.")
	}
	return nil
}

func ValidateCandidateMetadata(string) error {
	return commandError(CodeNotAvailable, "Sandboxed commands currently require macOS.")
}
