Task playbook: read-only planning.

Use this playbook when the user asks for a plan, design proposal, execution strategy or implementation approach without authorizing writes.

Steps:
1. Identify the exact artifact, scope level, RunMode and user goal from RunCommand.
2. Inspect current context only when it materially improves the plan.
3. Separate what can be known from current project state from assumptions.
4. Produce an executable plan with ordered steps, expected tools, validation points and risks.
5. Avoid progress language that implies execution has already happened.

Output requirements:
- Include summary, scope, planned steps, validation strategy, risks and next recommended action.
- Submit the complete Markdown proposal through create_plan; do not use finish(message).
- After create_plan, wait for explicit approval and do not execute or imply that execution has started.
- After revision feedback, replace the complete proposal through update_plan and request approval again.
