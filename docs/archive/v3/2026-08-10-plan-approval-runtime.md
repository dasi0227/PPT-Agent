# Plan Approval Runtime Design

Status: approved; refined after UI review

## Purpose

Turn Plan Mode from a read-only final-message generator into an approval gate
inside the existing single Run and single Harness Loop.

The Agent creates a complete plan, Runtime persists it, and the user chooses one
of three decisions. No decision takes effect until the user explicitly submits
the selected option.

## Product Decisions

- A user-visible task remains one Run from planning through execution.
- `RunIntent` is renamed to `RunMode`.
- `RuntimePhase` is renamed to `RunPhase`.
- The only mode transition is `plan -> execute`, performed by Runtime after an
  explicit user approval.
- `create_plan` persists the complete plan before approval and moves the Run to
  `waiting_input`; it does not call `finish`.
- `update_plan` replaces the complete proposal during planning and applies
  partial progress updates during execution.
- Runtime generates Plan IDs, step IDs, statuses, and revisions.
- `Plan.history` is removed. Event history and checkpoints remain the audit
  source.

## Run State Flow

```text
RunMode=plan, RunPhase=planning
  -> create_plan
  -> Plan.status=awaiting_approval
  -> RunPhase=waiting_input

  -> approve + submit
       -> Plan.status=active
       -> RunMode=execute
       -> RunPhase=executing
       -> continue the same Run and Agent Loop

  -> revise + feedback + submit
       -> RunMode=plan
       -> RunPhase=planning
       -> update_plan replaces the proposal
       -> RunPhase=waiting_input

  -> cancel + submit
       -> RunStatus=canceled
       -> RunPhase=terminal
```

The approval transition is atomic. Runtime validates the submitted Plan ID and
revision, records the decision, activates the Plan, changes the Run Mode,
switches the prompt module and recalculates tool disclosure before the next
Agent turn.

## Mode Transition Implementation

The current implementation repeatedly reads the immutable request intent from
`ContextPack.Command`. That is insufficient for a same-Run transition. The
authoritative current mode must move into Runtime state:

```text
RunState.mode RunMode
```

It is initialized from `RunCommand.mode` and then used by every dynamic policy:

- mode prompt selection;
- domain and control tool disclosure;
- write authorization checks;
- completion policy;
- context briefing and next-focus calculation;
- semantic review input;
- public progress and trace projection.

`RuntimeCheckpoint` also persists `mode`. Resume restores both the mode and the
complete Plan before another Agent turn is allowed.

On an approved decision, Runtime first prepares a complete Execute candidate
without mutating the authoritative Plan state. It then performs this sequence
without returning control to the model between steps:

```text
1. Validate waiting interaction, Plan ID and expected revision.
2. Prepare Plan.status=active, approved_revision and approved_content_hash.
3. Prepare RunState.mode=execute and the write RunSession.
4. Reassemble ContextPack from the original scope, instruction and options with mode=execute.
5. Rebuild ContextIndex, ContextBriefing and the context-bound domain tool registry.
6. Atomically persist the RunCommand mode, write-enabled ContextManifest and complete transition checkpoint.
7. Publish the prepared candidate as the authoritative RunState and set RunPhase=executing.
8. Emit plan.approval_answered, plan.updated and run.mode_changed.
9. Start the next Agent turn with execute prompt and write tools.
```

If candidate preparation or the durable commit fails, Runtime discards the
candidate and reopens the same approval interaction. The persisted Run remains
in plan mode with its read-only ContextManifest and awaiting Plan checkpoint;
no answered or mode-changed event is emitted. The persisted Run record and its
embedded RunCommand JSON are updated together, so Resume cannot reconstruct a
stale Plan command. The initial mode remains recoverable from `run.started`, so
an additional `initial_mode` field is not required.

Refreshing ContextPack is preferable to leaving its original read-only command
and manifest in place. It prevents the Agent from seeing contradictory
`plan/read_only` and `execute/write-enabled` authority descriptions after
approval.

## Plan Model

```json
{
  "plan_id": "plan_01",
  "revision": 4,
  "approved_revision": 3,
  "status": "active",
  "title": "Create the presentation",
  "content": "Complete user-facing Markdown plan",
  "steps": [
    {
      "id": "step_01",
      "title": "Create the outline",
      "status": "pending"
    }
  ]
}
```

`content` is the flexible, complete user-facing plan. `steps` are lightweight
progress anchors rather than a rigid representation of every explanation,
risk, or validation detail.

Runtime owns:

- `plan_id`;
- `revision` and `approved_revision`;
- step IDs;
- initial step statuses;
- valid status transitions;
- the Plan lifecycle status.

Every accepted Plan mutation produces a new Runtime revision. The user-facing
approval submission includes `expected_revision` to prevent a stale approval.

`Plan.history` is removed because public events and Runtime checkpoints already
preserve the change sequence. The latest Plan snapshot remains the only state
stored on the Plan itself.

## Model Tools

### `create_plan`

Disclosed during `RunMode=plan` when no Plan exists.

```json
{
  "title": "Create the presentation",
  "content": "Complete Markdown plan",
  "steps": [
    { "title": "Create the outline" },
    { "title": "Define the visual direction" }
  ]
}
```

The model does not provide IDs, statuses, or revisions. Runtime creates the
Plan, emits its public snapshot, requests approval, checkpoints, and waits for
input.

### `update_plan` during planning

Planning revisions use complete snapshot replacement:

```json
{
  "title": "Revised title",
  "content": "Complete revised Markdown plan",
  "steps": [
    { "title": "Revised first step" },
    { "title": "New second step" }
  ]
}
```

Runtime replaces the proposal content and proposal steps as one atomic update.
Pre-approval step IDs may be regenerated because no execution progress depends
on them yet.

This is more reliable for the Agent than a text diff or a before/after patch:

- the model only needs to produce the intended final Plan;
- no patch syntax, offsets, anchors, or operation ordering are required;
- omission and overlapping-edit failure modes are eliminated;
- Runtime can calculate any diagnostic diff from the previous snapshot.

### `update_plan` during execution

Execution progress uses partial patches:

```json
{
  "updates": [
    { "step_id": "step_01", "status": "completed" },
    { "step_id": "step_02", "status": "in_progress" }
  ]
}
```

Once the Plan is approved, title, content, and step structure are locked.
Sending the complete Plan on every progress change would waste tokens and risk
omitting steps or regressing completed work. Runtime applies the patch to the
current snapshot, validates transitions, increments the revision, and emits the
new public Plan snapshot.

The same public tool name may therefore have a dynamically disclosed schema:

```text
plan + proposal revision -> complete replacement schema
execute + active Plan    -> progress patch schema
```

Runtime keeps separate internal validators for proposal replacement and
progress updates.

## Complete Plan Visibility

The Agent must not depend on conversation history to remember the approved
Plan. Runtime injects the complete, exact Plan snapshot into every Agent turn.

The injection has two layers:

1. A non-compacted `approved_plan` Runtime prompt module contains `plan_id`,
   `approved_revision`, `title`, complete Markdown `content`, and every step.
2. `context_briefing` contains a short progress-oriented Plan summary for rapid
   focus, but never replaces the complete module.

The full module is rebuilt from `RunState.plan` on every turn, including
after message compaction. It is also restored from `RuntimeCheckpoint.Plan`
after process recovery. Therefore the Plan does not disappear when old chat
messages are summarized or provider continuations change.

Runtime records an `approved_content_hash` when approval is submitted. Before
each execute turn it verifies that the current title, content and step
structure still match that hash. Progress status fields are excluded from this
hash so `update_plan` can advance execution without invalidating approval.

The execute prompt explicitly treats the injected approved Plan as the
authoritative implementation contract. Execute-mode `update_plan` cannot alter
the hashed structural fields.

## Approval Interaction

The approval card renders:

```text
<icon> <title>

<content>

<approve> <revise> <cancel>

[feedback input when revise is selected]

<submit>
```

The three actions are mutually exclusive selections, not immediate buttons.
Nothing is sent when the user merely selects an option.

- No option selected: Submit is disabled.
- Approve selected: Submit sends `approve`.
- Revise selected: the feedback input is shown and required; Submit sends
  `revise + feedback`.
- Cancel selected: Submit sends `cancel`.
- The user may change the selection before submitting.
- No separate cancel-selection action is shown.
- After submission, the decision controls are locked and the card collapses to
  an icon, title, and result status.

The preview follows the current frontend design tokens and component density:

- Aptos-first sans-serif typography;
- workspace `#E9EDF2`, panel `#F8F9FB`, and surface white;
- accent `#2F67F6` and the existing success/danger tones;
- 10-12 px radii, restrained borders, and the current timeline spacing;
- the same focus-ring and reduced-motion behavior as the application.

## Public Events

The frontend reacts to product events rather than model tool names:

```text
plan.updated
plan.approval_requested
plan.approval_answered
run.mode_changed
```

`plan.approval_requested` carries the current Plan snapshot and interaction ID.
The submitted answer includes:

```json
{
  "interaction_id": "interaction_01",
  "plan_id": "plan_01",
  "expected_revision": 3,
  "decision": "revise",
  "feedback": "Add an explicit visual verification step"
}
```

Runtime rejects missing decisions, empty revision feedback, mismatched Plan IDs,
stale revisions, duplicate submissions, and submissions for a Run that is no
longer waiting.

## Prompt Policies

Plan Mode:

- remains read-only for project resources;
- uses `ask_user` only for a blocking missing decision;
- calls `create_plan` instead of `finish` for a normal completed proposal;
- waits after Plan creation;
- uses proposal-replacement `update_plan` after revision feedback;
- never claims execution before approval.

Execute Mode:

- receives the approved Plan in Runtime context;
- follows the approved structure;
- uses progress-patch `update_plan` to keep status current;
- cannot silently change Plan content or structure;
- uses `finish` only after execution and validation are complete.

## UI Preview

The approved interaction preview is stored at:

```text
docs/discuss/plan-approval-interaction-preview.html
```

It demonstrates option selection, conditional revision feedback, submit
validation, post-submit collapse, and the start of execution progress.

## Non-goals

- No second user-visible or hidden persistent Run is introduced.
- Plan steps do not become workflow DAG nodes.
- The frontend does not render raw `create_plan` tool calls.
- Direct editing of arbitrary Plan Markdown is not included in this iteration.
- Historical Plan revision browsing is not included.

## Acceptance Criteria

- Plan creation persists a Runtime Plan and waits without calling `finish`.
- All three decisions require an explicit Submit action.
- Revision feedback is required only when Revise is selected.
- Approval atomically moves the same Run from Plan Mode to Execute Mode.
- Planning updates atomically replace the complete proposal.
- Execution updates patch only approved step progress.
- Plan and step IDs and all revisions are Runtime-generated.
- `Plan.history` no longer exists.
- Runtime mode, full Plan and approval revision survive checkpoint resume.
- Every execute Agent turn receives the complete approved Plan outside the
  compactable message history.
- Public events replay to the same approval and progress UI state.
