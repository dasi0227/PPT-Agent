//go:build !darwin

package commandexec

func ValidateCandidateMetadata(string) error {
	return commandError(CodeNotAvailable, "Sandboxed commands currently require macOS.")
}
