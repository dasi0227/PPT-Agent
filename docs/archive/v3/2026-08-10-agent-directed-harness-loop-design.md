# Agent-Directed Harness Loop Design

Status: implemented

## Problem

The Runtime previously derived an `ExecutionStrategy` from hard-coded keywords,
slide counts and instruction length. `StrategyDecision` also carried synthetic
`confidence`, task-level `risk`, `reason` and `signals`.

This conflicted with the single Harness Loop model:

- semantic coordination choices were made by brittle rules instead of the Agent;
- `talk`, `ask`, `plan` and `execute` are expressed by `RunCommand.intent`;
- `execute` and `fulfill` only represented whether a Runtime Plan was required;
- `confidence` was hard-coded and had no consumer;
- task-level `risk` did not participate in tool authorization;
- the full decision was not checkpointed, so restored Strategy and Decision
  could disagree.

## Decision

Remove the Strategy layer completely.

The Runtime now has one authoritative authorization input:

```text
RunCommand.intent = talk | ask | plan | execute
```

Intent determines the maximum capability:

- `talk` and `ask` run in chat phase and are read-only;
- `plan` runs in planning phase and is read-only;
- `execute` runs in executing phase and is write-capable inside the declared
  RunScope.

There is no `StrategyDecision`, `StrategyRouter`, `ExecutionStrategy` or
`execute -> fulfill` transition.

## Agent-Owned Orchestration

`update_plan` is always disclosed during an execute run. The Agent decides
whether a plan is useful:

```text
simple task  -> read/write/verify directly
complex task -> create a plan, execute it and keep statuses current
scope grows  -> update the plan if useful, without gaining new authority
gate rejects -> repair directly or create/update a plan
```

A plan is optional. Once created, all of its steps must be completed before the
Completion Gate accepts `finish`.

The plan is not a workflow DAG, permission grant or scope expansion mechanism.

## Runtime-Owned Invariants

The deterministic Runtime continues to enforce:

- RunIntent and read/write authority;
- RunScope;
- phase-specific tool disclosure;
- tool-level risk;
- active write session requirements;
- schema, revision, hash and materialization checks;
- evidence freshness and Completion Gate policies;
- budgets, cancellation, idempotency and recovery.

Scope expansion still requires a revised command or user decision. Creating a
plan never authorizes an out-of-scope write.

## Contract Changes

Removed:

- `StrategyDecision`
- `DecisionSignal`
- `StrategyRouter`
- `ExecutionStrategy`
- `RuntimeCheckpoint.strategy`
- `AgentRequest.strategy`
- `SemanticReviewInput.strategy`
- `StructuredOutcome.strategy`
- Strategy task-risk propagation into `DomainToolInput`
- automatic upgrades based on keywords, target count, tool-call count or gate
  rejection count

Changed:

- Prompt mode selection uses RunIntent.
- The direct and fulfill prompts are merged into one execute Harness policy.
- Tool disclosure uses intent and phase.
- Completion policies use intent and optional Plan state.
- Checkpoint initialization is named `runtime_initialized`.

## Verification

Tests cover:

- read-only intents never disclose write tools;
- execute discloses both write tools and optional `update_plan`;
- Agent-created plans do not change loop identity or Runtime phase;
- execute runs may finish without a plan;
- an existing plan blocks completion until all steps are complete;
- checkpoints and outcomes no longer depend on Strategy;
- prompt modules are selected by intent.
