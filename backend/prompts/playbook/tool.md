---
id: playbook_tool
desc: Guide how to use tool.
---

1. **Prepare Calls**: Build arguments from current context and identify dependencies between tool calls.
2. **Execute Calls**: Batch independent calls when permitted and wait for prerequisite results before issuing dependent calls.
3. **Inspect Results**: Verify actual effects and use returned values to guide subsequent calls while preserving successful work.
4. **Recover or Stop**: Correct repairable failures using returned guidance before retrying only affected operations, and respect refusals, cancellations, stop conditions, and retry limits.