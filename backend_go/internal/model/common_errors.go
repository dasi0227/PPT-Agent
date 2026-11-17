package model

const (
	CodeNotFound             = "NOT_FOUND"
	CodeContentConflict      = "CONTENT_CONFLICT"
	CodeContentInvalid       = "CONTENT_INVALID"
	CodeEditAmbiguous        = "EDIT_AMBIGUOUS"
	CodeEditNotFound         = "EDIT_NOT_FOUND"
	CodeOutlineNotExist      = "OUTLINE_NOT_EXIST"
	CodeRunCanceled          = "RUN_CANCELED"
	CodeDependencyFailed     = "DEPENDENCY_FAILED"
	CodeResourceNotDisclosed = "RESOURCE_NOT_DISCLOSED"
	CodeToolArgumentInvalid  = "TOOL_ARGUMENT_INVALID"
	CodeToolCallInvalid      = "TOOL_CALL_INVALID"
	CodeIdempotencyKeyReused = "IDEMPOTENCY_KEY_REUSED"
	CodeOutOfScope           = "OUT_OF_SCOPE"
	CodePersistenceFailed    = "PERSISTENCE_FAILED"
	CodeInternal             = "INTERNAL"
)

var commonErrorCodes = []string{
	CodeNotFound, CodeContentConflict, CodeContentInvalid, CodeOutlineNotExist,
	CodeEditAmbiguous, CodeEditNotFound,
	CodeRunCanceled, CodeDependencyFailed, CodeResourceNotDisclosed,
	CodeToolArgumentInvalid, CodeToolCallInvalid, CodeIdempotencyKeyReused,
	CodeOutOfScope, CodePersistenceFailed, CodeInternal,
}

func IsCommonErrorCode(code string) bool {
	for _, common := range commonErrorCodes {
		if code == common {
			return true
		}
	}
	return false
}
