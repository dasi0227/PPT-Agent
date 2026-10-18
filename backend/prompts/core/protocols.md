---
id: core.protocols
description: Specify the protocols of the tools.
---
## Result Status

Every tool call result includes a shared `status` field:
- `success`: The operation has been authorized and completed successfully.
- `refuse`: The user declined the proposed operation.
- `error`: Validation, execution or persistence failed.

## Error Fields

Every tool call result with `error` status includes these shared fields:
- `code`: Error identifier.
- `category`: Recovery classification.
	- `repairable`: May be resolved by correction or retry.
	- `canceled`: Canceled by runtime.
	- `terminal`: May not be resolved under current conditions.
- `reason`: Cause of failure.
- `measure`: Suggested remedy or action.

## Shared Error Codes

The following shared error codes identify failures common to multiple tools:
- `INTERNAL`: An internal failure prevents safe continuation.
- `RUN_CANCELED`: The run is canceled.
- `NOT_FOUND`: The target does not exist.
- `TOOL_ARGUMENT_INVALID`: Arguments do not match the tool schema.
- `TOOL_CALL_INVALID`: The call violates invocation rules.
- `OUT_OF_SCOPE`: The operation exceeds the authorized scope.
- `DEPENDENCY_FAILED`: Prerequisites do not take effect.
- `RESOURCE_NOT_DISCLOSED`: The resource is not exposed.
- `IDEMPOTENCY_KEY_REUSED`: A call ID is reused for a different request.
- `EDIT_AMBIGUOUS`: `old_text` matches more than once.
- `EDIT_NOT_FOUND`: `old_text` has no match.
- `CONTENT_INVALID`: Content violates syntax, schema, or constraints.
- `CONTENT_CONFLICT`: Saved content differs from the expected version.
- `OUTLINE_NOT_EXIST`: The outline does not exist.
- `PERSISTENCE_FAILED`: The changes can not be safely saved.
