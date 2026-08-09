# Slide Spec and Materialization Contract Implementation Plan

## Goal

Implement the approved slide spec contract, make JSON files authoritative for
current revisions, and move per-slide HTML materialization metadata from SQLite
to `slides/<slide_id>/materialization.json`.

## Task 1: Domain Schemas and Types

Files:

- `backend/schemas/slide-spec.schema.json`
- `backend/schemas/materialization.schema.json`
- `backend/schemas/schemas.go`
- `backend/schemas/schemas_test.go`
- `backend/internal/spec/types.go`
- `backend/internal/spec/validate.go`
- `backend/internal/spec/validate_test.go`
- `backend/internal/model/slidepath.go`

Steps:

1. Replace the legacy slide fields with `version`, `project`, `role`, `title`,
   `key_message`, `elements` and optional `layout`.
2. Define the element type enum and exact two-field item shape.
3. Mark all identity, placement and bookkeeping fields Runtime-owned.
4. Add the Runtime-only materialization schema and Go types.
5. Add schema validation and path helpers.
6. Add positive and negative tests, including Agent contract field stripping.

## Task 2: Runtime Normalization and Context

Files:

- `backend/internal/workflow/ppt_helpers.go`
- `backend/internal/workflow/completion.go`
- `backend/internal/workflow/evidence.go`
- `backend/internal/contextengine/loaders.go`
- `backend/internal/contextengine/assembler.go`
- `backend/internal/contextengine/types.go`

Steps:

1. Remove `source_outline_revision` and speaker-note handling.
2. Derive asset search queries from `elements[type=asset].intent`.
3. Split render proof identity into artifact and source hashes.
4. Load HTML revision and source revisions from materialization files.
5. Derive materialization state from current JSON revisions and exact hashes.
6. Treat every semantic spec change as HTML-affecting.

## Task 3: Service Commit and Read Paths

Files:

- `backend/internal/service/workflow_commit.go`
- `backend/internal/service/spec.go`
- `backend/internal/service/slide_content.go`
- `backend/internal/service/slide.go`
- `backend/internal/model/artifact_commit.go`
- `backend/internal/model/project.go`
- `backend/internal/model/slide.go`

Steps:

1. Stop writing current revision mirrors to SQLite.
2. Project outline, design and spec revisions from files for API compatibility.
3. Project HTML/source revisions from materialization files.
4. Verify render proofs against exact current files.
5. Write materialization files atomically and restore them if DB commit fails.
6. Remove materialization on HTML rollback.

## Task 4: SQLite Migration

Files:

- `backend/migrations/0008_file_materialization.sql`
- `backend/migrations/migrations_test.go`
- `backend/internal/store/store.go`
- `backend/internal/store/sqlite/po.go`
- `backend/internal/store/sqlite/run_store.go`
- `backend/internal/store/sqlite/slide_store.go`
- `backend/internal/store/sqlite/layout_migration.go`
- new materialization migration code and tests

Steps:

1. Back up legacy slide materialization columns.
2. Rebuild `projects` and `slides` without revision mirrors.
3. Update PO and store methods.
4. Bump project filesystem layout version.
5. Normalize current specs and snapshots to the new contract.
6. Generate materialization files from valid legacy metadata.
7. Remove the backup table only after filesystem migration succeeds.

## Task 5: Frontend and Prompt Contracts

Files:

- `frontend/src/api/types.ts`
- `frontend/src/features/viewer/SlideSpecCard.tsx`
- related frontend tests and fixtures
- `backend/prompts/runtime/resources/resource_contracts.md`
- `backend/prompts/runtime/playbooks/spec_edit.md`
- `backend/prompts/runtime/playbooks/empty_deck_generation.md`
- `backend/prompts/runtime/business/ppt_business_policy.md`

Steps:

1. Update TypeScript `SlideSpec`.
2. Render element type and intent in the spec card.
3. Replace legacy fixture fields.
4. Update Runtime-owned field naming and slide-spec responsibilities.
5. Remove stale outline/design contract wording.

## Task 6: Verification

Run:

```bash
cd backend
go test ./internal/model ./internal/store/sqlite ./internal/service ./internal/workflow ./internal/spec ./schemas ./internal/httpapi ./internal/contextengine ./internal/designsystem ./internal/asset ./internal/config ./migrations
go test ./...

cd frontend
pnpm tsc --noEmit
pnpm test -- --run
pnpm build
```

Also run:

```bash
git diff --check
git status --short
```

Fix all regressions caused by this change. Report unrelated pre-existing failures
only after confirming the affected packages and build pass.

## Task 7: Commit

1. Remove the temporary TODO after all decisions are represented by the design
   document and implementation.
2. Review the complete diff for unrelated changes.
3. Commit all implementation and documentation in one commit.
