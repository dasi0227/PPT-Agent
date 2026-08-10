# Plan Approval Runtime Design

Status: proposed for user review

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

## Plan Model

```json
{
  "plan_id": "plan_01",
  "revision": 3,
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
- Public events replay to the same approval and progress UI state.
