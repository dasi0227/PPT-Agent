---
id: tool_ask_user
desc: Ask the user for missing information or a decision.
---

Ask the user for information or a decision that materially affects the task and cannot be resolved from available context. Submit this call alone and wait for the result. Questions in the same call must be independently answerable; defer questions that depend on unanswered ones. Provide at most three meaningful options per question; one or two are sufficient when appropriate. Omit `options` to present a free-text input instead. A successful call returns `answers` in the submitted question order. Each answered question contains the original selected option label or the user’s verbatim response. A skipped question returns `{"state":"skipped"}` and supplies no answer or approval.