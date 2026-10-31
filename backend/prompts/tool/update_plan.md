---
id: tool_update_plan
desc: Update execution statuses in the approved plan.
---

Update execution statuses of existing steps in the approved active plan, using step IDs from `<current_plan>` within `<runtime_context>`. Submit this call alone. Submit each changed step once and at most one step may be `processing`. Omitted steps retain their status. Completed steps cannot revert to another status. The plan completes automatically when all steps are completed. A successful call returns only `status=success`.
