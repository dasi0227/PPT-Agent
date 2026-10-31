---
id: tool_request_privilege
desc: Request authorization to work on additional slides.
---

Request authorization to edit or render additional slides outside the current scope. Submit this call alone and wait for the result. Approval updates the authorized slide IDs in `<run_state>` within `<runtime_context>`. A refusal leaves the scope unchanged and may include the user’s reason in `feedback`.