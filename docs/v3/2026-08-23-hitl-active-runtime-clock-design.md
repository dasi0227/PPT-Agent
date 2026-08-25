# HITL Active Runtime Clock Design

## Scope

Exclude user decision latency from the Agent runtime duration budget and from the public execution duration shown in the timeline. The change applies to ordinary `ask_user` questions and Plan Approval waits, including repeated waits and checkpoint recovery.

## Decisions

- Runtime duration means active Agent execution time, not wall-clock lifetime.
- The active clock runs while the Runtime is reasoning, calling tools, processing an answer, checkpointing active work, or completing a transition.
- The active clock pauses immediately before Runtime blocks for HITL input and resumes immediately after a valid input signal returns.
- Cancellation or failure while blocked reports the already accumulated active duration and does not charge the blocked interval.
- `RuntimeBudget.MaxDuration` and public `run.finished.duration_ms` use the same active clock.
- Checkpoints persist `active_duration_ms` and `waiting_duration_ms`; recovery resumes both accumulated clocks instead of resetting duration accounting.
- Engine fallback terminal events use the Runtime outcome duration when it is available. Scheduler-only failures before Runtime starts keep their existing wall-clock duration.
- Trace data records active, wall, and inferred waiting durations for diagnosis. The public event contract remains unchanged.

## Data Flow

1. Runtime initializes an active clock and restores any checkpointed active duration.
2. Before `ask_user` or `AskPlanApproval` blocks, Runtime pauses the clock.
3. On a user answer, Runtime resumes the clock before validating and processing the answer.
4. Budget checks read the active clock.
5. Checkpoints snapshot the active duration.
6. Terminal outcomes and `run.finished` events expose the active duration.

## Non-goals

- Changing the turn budget.
- Adding an automatic deadline for how long a user may leave a HITL card unanswered.
- Changing frontend duration formatting or the public SSE schema.
- Changing provider, tool, render-worker, or project-lock timeouts.

## Acceptance Criteria

1. A HITL wait longer than `MaxDuration` does not fail the Run when active execution remains within budget.
2. Active execution that exceeds `MaxDuration` still fails with `RUNTIME_BUDGET_EXCEEDED`.
3. Questions and Plan Approval both exclude all user waiting intervals.
4. Terminal `duration_ms` excludes HITL wait time for completed, failed, and canceled Runtime outcomes.
5. Active duration survives checkpoint serialization and recovery.
6. Engine fallback terminal events prefer the active duration carried by a Runtime outcome.
7. Tests cover ordinary questions, plan approval, recovery, budget exhaustion, and terminal duration propagation.
