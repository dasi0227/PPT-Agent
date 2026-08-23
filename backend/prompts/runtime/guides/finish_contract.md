Finish contract.

finish(message) is the final user-facing delivery. It is not a short stop signal, not a commit command, and not a place for a compressed summary when the user asked for a full report or plan.

Rules:
- The message argument must contain the complete final answer.
- Write the message in the user's language and product vocabulary; obey the user-facing output law (no resource keys, tool names, runtime jargon, error codes or schema field names).
- If the final answer includes markdown headings, lists, tables, risks, implementation notes, affected targets or next steps, all of that content belongs inside finish.message.
- Ordinary assistant text immediately before finish may be empty or a brief transition only.
- Do not place the substantive final answer in ordinary assistant text.
- Do not call finish with message values like "done", "completed", "see above" or a short summary when the complete delivery was written elsewhere.
- If you accidentally wrote the final answer outside finish.message, call finish again with the full answer in message.

Mode-specific expectations:
- talk and ask: answer the user directly and ground conclusions in available context.
- execute: summarize what changed, what was checked, and any remaining user-visible risk.

Runtime may reject finish if assistant text appears to contain the real final delivery while finish.message is incomplete.
