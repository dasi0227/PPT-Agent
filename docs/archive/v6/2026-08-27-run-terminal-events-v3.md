# Run Terminal Events v3

Date: 2026-08-27
Status: Accepted for development

## Decision

Run terminal events no longer use `run.finished` plus an inner `status` field.
The terminal result is expressed by the outer event name:

```text
run.completed
run.failed
run.error
run.canceled
```

All four events share the same payload shape:

```json
{
  "schema_version": 3,
  "run_id": "run_123",
  "occurred_at": "2026-08-27T06:30:00.000Z",
  "duration_ms": 12000,
  "affected_targets": [],
  "error": null,
  "trace_id": "run_123"
}
```

## Semantics

| Event | Meaning | Error |
|---|---|---|
| `run.completed` | LLM called finish and backend completion review plus commit checks passed. | `null` |
| `run.failed` | Runtime closed normally, but rules, budgets, gates, or known dependencies determined the task failed. | `PublicError` |
| `run.error` | Engineering runtime error or unsafe runtime state. The run cannot be trusted to continue. | `PublicError` |
| `run.canceled` | User or system cancellation. | usually `null` |

`affected_targets` contains only targets that the backend can confirm as applied or safely identify. It must not include merely attempted writes.

`trace_id` is a safe support handle. It must not expose paths, prompts, provider payloads, tokens, secrets, or raw generated content.

## Frontend Rules

- `run.completed` updates the final message duration and affected targets.
- `run.failed` creates a terminal notice using the failure style.
- `run.error` creates a distinct runtime error notice using the system error style.
- `run.canceled` creates a cancellation notice.
- Any terminal event clears the live loading row and closes the active SSE session.
