package commandexec

import (
	"errors"
	"fmt"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

const (
	CodeParseInvalid = "COMMAND_PARSE_INVALID"
	CodeSyntaxDenied = "COMMAND_SYNTAX_DENIED"
	CodeNotAllowed   = "COMMAND_NOT_ALLOWED"
	CodeNotAvailable = "COMMAND_NOT_AVAILABLE"
	CodeFlagDenied   = "COMMAND_FLAG_DENIED"
	CodePathInvalid  = "COMMAND_PATH_INVALID"
	CodeExitNonzero  = "COMMAND_EXIT_NONZERO"
	CodeTimeout      = "COMMAND_TIMEOUT"
	CodeOutputLimit  = "COMMAND_OUTPUT_LIMIT"
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
	var typed *Error
	if errors.As(err, &typed) {
		return typed.Code
	}
	return model.AsAgentError(err, model.CodeInternal, "run_command").Code
}
