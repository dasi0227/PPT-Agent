package commandexec

import (
	"errors"
	"fmt"
	"os"

	"github.com/dasi0227/PPT-Agent/backend/internal/model"
)

const (
	CodeInvalid      = "COMMAND_INVALID"
	CodeNotAllowed   = "COMMAND_NOT_ALLOWED"
	CodeNotAvailable = "COMMAND_NOT_AVAILABLE"
	CodeExitNonzero  = "COMMAND_EXIT_NONZERO"
	CodeTimeout      = "COMMAND_TIMEOUT"
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
	if errors.Is(err, os.ErrNotExist) {
		return model.CodeNotFound
	}
	return model.AsAgentError(err, model.CodeInternal, "run_command").Code
}
