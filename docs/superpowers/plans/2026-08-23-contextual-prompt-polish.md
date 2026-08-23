# Contextual Prompt Polish Implementation Plan

Status: implemented

Design:
`docs/superpowers/specs/2026-08-23-contextual-prompt-polish-design.md`

## 1. Add the Read-only Polish Context Projection

Files:

- `backend/internal/contextengine/types.go`
- `backend/internal/contextengine/assembler.go`
- `backend/internal/contextengine/compiler.go` or a focused new projection file
- `backend/internal/contextengine/*_test.go`

Actions:

- add a typed `PolishContext` request/result owned by `contextengine`;
- reuse existing project, outline, design, target-slide, memory and recent-turn loaders rather than re-reading artifacts in HTTP/service code;
- apply a small Polish-specific token budget and deterministic relevance ordering;
- omit ContextRef registration, complete HTML, asset indexes, provider state, tools and Run state;
- mark all loaded project content as untrusted reference data in the compiled projection;
- validate project/thread ownership and slide target membership;
- test empty projects, deck/slide scopes, target-page resolution, thread context, budget trimming, cross-project thread rejection and absence of ContextRefs.

## 2. Add the Polish Prompt Registry and Service

Files:

- `backend/prompts/polish/registry.go`
- `backend/prompts/polish/*.md`
- `backend/internal/service/polish.go`
- `backend/internal/service/polish_test.go`
- `backend/internal/model/agent_error.go` if a dedicated empty-output code is required

Actions:

- create a versioned embedded Polish prompt registry, separate from Runtime prompts;
- encode intent preservation, context precedence, untrusted-context handling, Design Intent Normalization and output-only rules;
- include only original, concise translation examples inspired by the identified public Skills; do not copy their content wholesale;
- resolve the requested configured model via the existing LLM registry and invoke its adapter once with no tools, image resolver or continuation;
- bound the request with the Polish deadline and validate input length, trimmed instruction and model/profile selection;
- return `polished_instruction`, byte-comparison `changed`, and prompt version;
- test correct prompt/context assembly, ordinary text preservation, visual-intent normalization, no invented facts, empty output handling, provider failures and cancellation.

## 3. Expose One Project-scoped HTTP API

Files:

- `backend/internal/httpapi/polish_handler.go`
- `backend/internal/httpapi/router.go`
- `backend/internal/httpapi/*_test.go`
- `backend/cmd/server/wire.go`
- `backend/cmd/server/wire_gen.go`

Actions:

- add `POST /api/v1/projects/:project_id/polish` and one request/response contract;
- accept instruction, optional thread ID, Composer scope/mode and selected model; derive project identity from the route;
- use the existing API error envelope and reject malformed, oversized, unknown-profile, invalid-scope and cross-project-thread requests;
- keep the handler synchronous and ensure no dependency on `RunService`, `run.Engine`, event streams or persistent operation records;
- wire the new handler/service through Wire and regenerate `wire_gen.go`;
- add handler tests for response shape, request validation, ownership and error mapping.

## 4. Replace Frontend Enhance with Polish

Files:

- `frontend/src/api/polish.ts`
- `frontend/src/api/types.ts`
- `frontend/src/features/agent/CommandComposer.tsx`
- `frontend/src/features/agent/CommandComposer.test.tsx`
- `frontend/src/index.css`

Actions:

- replace Enhance naming, labels, ARIA copy, local variables and CSS class names with Polish / 润色表达;
- add the project-scoped Polish client request and build its payload from the current Composer selection, selected model, active project and optional active thread;
- replace the fixed timer with an abortable latest-request flow;
- disable editing/send only while the current Polish request is active;
- on a valid latest success replace the text and restore focus at the end; preserve text/selection on error, cancellation, stale response or unmount;
- retain the restrained busy animation with a reduced-motion path;
- prove in tests that Polish neither creates a Run nor changes steering behavior; submitting afterward continues to use existing create/steer paths.

## 5. Verify the Contract and Regression Boundaries

Commands:

```text
gofmt changed Go files
go test ./backend/internal/contextengine ./backend/internal/service ./backend/internal/httpapi
go test ./backend/... -timeout 90s
pnpm --dir frontend tsc --noEmit
pnpm --dir frontend test -- --run
pnpm --dir frontend build
git diff --check
```

Manual checks:

- a target-slide request such as “把这一页做得更有冲击力” returns a target-aware instruction;
- a visual/motion phrase becomes concrete design language without invented libraries or numeric claims;
- an active Run continues untouched while Polish is pending and after it succeeds;
- an unavailable provider leaves the original Composer text intact;
- narrow Composer layouts and `prefers-reduced-motion` remain usable.

## 6. Documentation and Commit

Files:

- `docs/superpowers/specs/2026-08-23-contextual-prompt-polish-design.md`
- `docs/superpowers/plans/2026-08-23-contextual-prompt-polish.md`

Actions:

- keep the design and implementation plan synchronized with the final API and prompt version;
- review for stale `Enhance` naming, duplicated Runtime semantics, placeholder text and conflicting ownership statements;
- commit documentation and implementation in coherent commits using the repository commit convention.
