---
id: tool_create_plan
desc: Submit a complete execution plan for user approval.
---

Create or fully replace an unapproved plan and submit it for user approval. Submit this call alone. Organize the complete Markdown plan in `content` under `## Request Restatement`, `## Proposed Approach`, `## Ordered Steps`, `## Intended Outcome`, and `## Acceptance Criteria`, in that order, translating the headings to match the user's language. Include the execution steps in the plan body and provide a concise action or deliverable title for each in `steps`, in the same order and with no added or omitted work. When revisions are requested, resubmit the complete plan and all steps. Approval returns `status=success`. A refusal returns `status=refuse` and may include the user's reason or revision instructions in `feedback`. The plan and its assigned step IDs are automatically supplied in `<current_plan>` within `<runtime_context>`.
