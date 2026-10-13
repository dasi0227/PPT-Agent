package model

const (
	CodeRunCanceled          = "RUN_CANCELED"
	CodeDependencyFailed     = "DEPENDENCY_FAILED"
	CodeResourceNotDisclosed = "RESOURCE_NOT_DISCLOSED"
	CodeToolArgumentInvalid  = "TOOL_ARGUMENT_INVALID"
	CodeToolCallInvalid      = "TOOL_CALL_INVALID"
	CodeIdempotencyKeyReused = "IDEMPOTENCY_KEY_REUSED"
	CodeTargetOutOfScope     = "TARGET_OUT_OF_SCOPE"
	CodePersistenceFailed    = "PERSISTENCE_FAILED"
	CodeInternal             = "INTERNAL"
)

var commonErrorCodes = []string{
	CodeRunCanceled, CodeDependencyFailed, CodeResourceNotDisclosed,
	CodeToolArgumentInvalid, CodeToolCallInvalid, CodeIdempotencyKeyReused,
	CodeTargetOutOfScope, CodePersistenceFailed, CodeInternal,
}

func CommonErrorCodes() []string {
	return append([]string(nil), commonErrorCodes...)
}

func IsCommonErrorCode(code string) bool {
	for _, common := range commonErrorCodes {
		if code == common {
			return true
		}
	}
	return false
}
