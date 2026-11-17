---
id: core_protocols
desc: Specify the protocols of the tools.
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
