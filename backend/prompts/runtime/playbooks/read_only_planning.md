Task playbook: read-only planning.

Use this playbook when the user asks for a plan, design proposal, execution strategy or implementation approach without authorizing writes.

Steps:
1. Identify the exact artifact, scope level, RunIntent and user goal from RunCommand.
2. Inspect current context only when it materially improves the plan.
3. Separate what can be known from current project state from assumptions.
4. Produce an executable plan with ordered steps, expected tools, validation points and risks.
5. Avoid progress language that implies execution has already happened.

Output requirements:
- Include summary, scope, planned steps, validation strategy, risks and next recommended action.
- If the user requested code or PPT changes, state that execution must happen in execute mode.
- Put the full plan in finish(message).
