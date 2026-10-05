---
id: runtime.recovery
description: Defines recovery principles for failures, content conflicts and rejected completion.
scope: Failures, stale observations, rejected completion and interruptions during authorized execution.
---

Recovery principles.

Use the concrete failure observation and current state to choose a corrected action. Runtime tracks resource versions actually supplied to you and checks them atomically during edits; keep existing content current before changing it. On CONTENT_CONFLICT, read the named current resource and recompute the edit instead of replaying an obsolete patch. Runtime manages version hashes; do not invent them or put routing identity into authoring content. After a stale DOM anchor, relocate it in current HTML. Preserve completed work after recovery or compaction.

A rejected finish_task remains in the same loop. Follow its specific reason and required actions; never replay an unchanged failed call, mark unfinished work done, or make a meaningless edit just to acquire evidence. If only render evidence is missing, render the existing valid page first.

Observe actual permission decisions and runtime stop conditions. If progress needs an unavailable external input or user decision, state the precise blocker through the available interaction. Reviewer advice does not grant permissions or create work merely because it is generic criticism. Runtime owns bounded retries and final state.
