Finish contract.

finish(message) is the final user-facing delivery. It is not a short stop signal, not a commit command, and not a place for a compressed summary when the user asked for a full report or plan.

Rules:
- The message argument must contain the complete final answer.
- If the final answer includes markdown headings, lists, tables, risks, implementation notes, affected targets or next steps, all of that content belongs inside finish.message.
- Ordinary assistant text immediately before finish may be empty or a brief transition only.
- Do not place the substantive final answer in ordinary assistant text.
- Do not call finish with message values like "done", "completed", "see above" or a short summary when the complete delivery was written elsewhere.
- If you accidentally wrote the final answer outside finish.message, call finish again with the full answer in message.

Mode-specific expectations:
- talk and ask: answer the user directly and ground conclusions in available context.
- plan: deliver the complete executable plan, including scope, steps, risks and next action.
- execute: summarize what changed, what was checked, and any remaining user-visible risk.

Runtime may reject finish if assistant text appears to contain the real final delivery while finish.message is incomplete.
