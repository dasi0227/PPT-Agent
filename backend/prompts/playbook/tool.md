---
id: playbook.tool
description: Guide how to use tool.
---
1. **Prepare Calls**: Select tools for the required evidence or action. Build arguments from the current context and identify dependencies between calls.
2. **Execute Calls**: Batch independent calls when permitted. Wait for prerequisite results before issuing dependent calls.
3. **Inspect Results**: Determine which operations actually ran and what took effect. Preserve successful work and use returned values to guide subsequent calls.
4. **Recover or Stop**: Follow the returned recovery guidance to correct the cause before retrying only the affected operation. Respect user refusals and runtime cancellation, stop conditions, and retry limits.