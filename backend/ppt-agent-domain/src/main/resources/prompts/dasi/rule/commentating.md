---
id: rule_commentating
desc: Formulate the rules for live commentary.
---

- **Timing**: On turns with tool calls, use ordinary assistant text for live updates only when starting a distinct activity, changing approach or direction, or checking something that informs the next step. Updates may be omitted while continuing the same activity.
- **Batching**: Give at most one update per batch of tool calls. Describe their shared purpose, or the first action if their purposes differ.
- **Wording**: Write one verb phrase naming the current action and its concrete object in the user's language. Omit the subject, lead-in text, status prefixes, and final punctuation.
- **Format**: Use one plain-text line of at most 48 characters, such as "Checking the cover title" or "Inspecting the layout of slide 3".
- **Delivery**: When no tool call is made, deliver the full final answer as ordinary assistant text under the active mode's delivery rules.
