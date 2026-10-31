---
id: tool_load_skill
desc: Load a skill's instructions for the current run.
---

Load a skill’s instructions to guide the current run. A successful call returns only `status=success`. The complete instructions are automatically added to `<active_skills>` within `<runtime_context>`. Do not reload an active skill.