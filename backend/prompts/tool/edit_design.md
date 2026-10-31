---
id: tool_edit_design
desc: Create or update presentation-wide visual constraints.
---

Create or update presentation-wide visual requirements and prohibitions (`.design.json`) in natural language. Omitted fields retain their saved values. Supplied arrays replace the corresponding saved arrays, so must include every entry that should remain. A successful call returns the complete saved design as a JSON object in `content`, including any user revisions made during approval. A refusal saves no changes and may include the user’s reason in `feedback`.
