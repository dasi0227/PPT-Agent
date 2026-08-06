Completion repair guide.

If finish is rejected, do not restart, summarize excuses, or stop. Continue in the same ReAct loop and repair the exact issue.

Issue handling:
- SCHEMA_EVIDENCE_REQUIRED: rewrite or repair the JSON resource with write_ppt so schema evidence is fresh for the target hash.
- STATIC_EVIDENCE_REQUIRED: edit or rewrite the affected HTML so static evidence matches the latest render source hash.
- VISUAL_EVIDENCE_REQUIRED: call render_slide for the affected slide. Inspect overflow, clipping, console errors, failed resources and font status. Repair blocking issues and render again.
- REFERENCE_EVIDENCE_REQUIRED: repair outline/spec references and rewrite the relevant resource so reference integrity evidence is fresh.
- SLIDE_HTML_SYNC_REQUIRED: update the HTML that materializes the changed slide spec, then render the slide.
- PLAN_INCOMPLETE: update the disclosed execution plan so completed work is marked completed and pending, in_progress or failed work is resolved.
- TARGET_OUT_OF_SCOPE: stop using the unauthorized target. Work only inside the current scope or ask_user if a user decision is needed.
- REQUIREMENT_UNADDRESSED: inspect the requirement ledger and complete the missing user requirement before finish.
- FINISH_CONTRACT_VIOLATION: resubmit finish with the complete final answer in finish.message.
- TOOLS_RUNNING, FINISH_NOT_ALLOWED or RUN_CANCELED: respect Runtime state. Do not attempt to bypass the gate.

Repair discipline:
- Prefer the minimal correction that makes the evidence fresh.
- Do not mark a plan step completed merely because you attempted it. Mark it completed only after the corresponding work is actually done.
- Do not call finish repeatedly without new evidence, plan progress or requirement coverage.
