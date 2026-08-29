package commandexec

import "fmt"

const (
	CodeParseInvalid       = "COMMAND_PARSE_INVALID"
	CodeSyntaxDenied       = "COMMAND_SYNTAX_DENIED"
	CodeNotAllowed         = "COMMAND_NOT_ALLOWED"
	CodeNotAvailable       = "COMMAND_NOT_AVAILABLE"
	CodeFlagDenied         = "COMMAND_FLAG_DENIED"
	CodePathOutsideProject = "COMMAND_PATH_OUTSIDE_PROJECT"
	CodePathInvalid        = "COMMAND_PATH_INVALID"
	CodeSensitiveDenied    = "COMMAND_SENSITIVE_READ_DENIED"
	CodeExitNonzero        = "COMMAND_EXIT_NONZERO"
	CodeTimeout            = "COMMAND_TIMEOUT"
	CodeOutputLimit        = "COMMAND_OUTPUT_LIMIT"
	CodeExecFailed         = "COMMAND_EXEC_FAILED"
	CodeInvariantViolation = "COMMAND_INVARIANT_VIOLATION"
)

type Error struct {
	Code    string
	Message string
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func commandError(code, message string) error {
	return &Error{Code: code, Message: message}
}

func ErrorCode(err error) string {
	if typed, ok := err.(*Error); ok {
		return typed.Code
	}
	return CodeExecFailed
}
