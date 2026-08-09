# Run Command Contract Implementation Plan

Design:
`docs/superpowers/specs/2026-08-10-run-command-contract-design.md`

## 1. Replace the Domain Contract

Files:

- `backend/internal/model/run_command.go`
- `backend/internal/model/run_command_test.go`
- `backend/internal/model/run_entity.go`

Actions:

- replace `WorkSpec` with `RunCommand`;
- replace `RunTarget` with `RunScope`;
- replace `TargetLevel` with `ScopeLevel`;
- replace `InteractionIntent` with `RunIntent`;
- remove `RunInteraction`;
- replace `presentation` with `ppt`;
- define validated language and range enums;
- rename Run and CreateRunParams ownership fields to `Command`;
- add validation tests for all enum and conditional rules.

## 2. Migrate SQLite Persistence

Files:

- `backend/migrations/0011_run_command_contract.sql`
- `backend/migrations/migrations_test.go`
- `backend/internal/store/sqlite/po.go`
- `backend/internal/store/sqlite/run_store.go`
- relevant store tests

Actions:

- rebuild `runs` with scope/intent/run-command columns;
- transform existing command JSON and artifact values;
- map legacy exact slide counts into range buckets;
- remove legacy option keys;
- update PO conversion and insert SQL;
- test schema columns, JSON migration and current round-trip.

## 3. Update API, Events and Context

Files:

- `backend/internal/httpapi/run_handler.go`
- `backend/internal/model/public_event.go`
- `backend/internal/run/engine.go`
- `backend/internal/run/bus.go`
- `backend/internal/contextengine/*`

Actions:

- expose `scope` and `intent` in create/get Run APIs;
- emit public event schema v3;
- write scope/intent into new history entries;
- rename ContextPack and ContextRequest fields to `Command`;
- bump ContextPack schema to 2.0;
- rename the required context segment and PPT profile IDs;
- update tests for the new serialized shapes.

## 4. Simplify Workflow Authorization

Files:

- `backend/internal/workflow/tools.go`
- `backend/internal/workflow/context_retrieval.go`
- `backend/internal/workflow/runtime.go`
- `backend/internal/workflow/completion.go`
- `backend/internal/workflow/semantic_reviewer.go`
- `backend/internal/workflow/requirement_ledger.go`
- related workflow tests

Actions:

- remove the internal `Scope` wrapper;
- introduce pure `AllowsRead`, `AllowsWrite` and `AllowsArtifact` functions;
- pass `model.RunScope` through Runtime, tools, retrieval and Completion Gate;
- consume `RunCommand.Intent` directly;
- add language/range completion policies;
- add command options to the requirement ledger and prompts;
- verify spec/PPT and slide/deck authorization.

## 5. Migrate Frontend Runtime State

Files:

- `frontend/src/api/types.ts`
- `frontend/src/api/sse.ts`
- `frontend/src/features/agent/*`
- `frontend/src/stores/composerStore.ts`
- `frontend/src/stores/runStore.ts`
- related frontend tests

Actions:

- replace target/interaction request and response fields with scope/intent;
- replace presentation with ppt;
- update composer shortcuts and capability checks;
- update session and timeline state;
- reject pre-v3 SSE events and legacy history entries;
- update labels, tests and fixtures.

## 6. Synchronize Prompts and Documentation

Files:

- `backend/prompts/runtime/**`
- `backend/prompts/semantic_reviewer/**`
- `docs/learn/ppt-agent-architecture-guide.html`
- `tmp/runtime-schema.md`
- `tmp/runtime-schema-todo.md`

Actions:

- replace WorkSpec/target/interaction language with RunCommand/scope/intent;
- document language conflict and range behavior;
- update Runtime schema examples;
- mark TODO items 1 through 8 implemented.

## 7. Verify and Commit

Commands:

```text
gofmt changed Go files
go test key backend packages
go test ./... -timeout 90s
pnpm tsc --noEmit
pnpm test
pnpm build
git diff --check
```

Expected known unrelated failures:

- `internal/llm` DeepSeek cancellation test may time out while closing an
  active `httptest.Server` connection;

After reviewing the final diff, create one commit containing the design,
implementation plan, migrations, code, tests and documentation.
