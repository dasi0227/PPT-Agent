# Canonical PPT Architecture (v4)

This document describes the only supported development contract after the 2026-08-26 breaking refactor.

## Persistence and identity

- `deck.json` owns deck intent, canvas, and numbering policy.
- `outline.json` is a section/subsection tree and is the sole authority for slide membership and order.
- `design.json` owns the global visual direction and shared chrome.
- `slides/<slide_id>/spec.json` owns one slide's semantic design and never stores hierarchy or ordering.
- `slides/<slide_id>/index.html` contains only the Agent-authored slide body.
- `slides/<slide_id>/materialization.json` independently proves the HTML body source and deterministic Runtime frame context.
- Runtime creates all stable `sec_`, `sub_`, and `sli_` IDs. Agent mutations provide temporary `client_ref` values only.

## Mutation and API contract

All deck, outline, design, slide-spec, and slide-HTML changes flow through the discriminated `mutate_ppt` domain operation set. The same mutation service backs Agent run overlays and `POST /api/v1/projects/:id/mutations`. `GET /api/v1/projects/:id/content` returns the canonical deck/outline/design/slide map snapshot; during a run it returns the current active overlay.

There is no structure-specific endpoint or separately persisted page-order array. Direct UI mutations are rejected while a run is active, and successful UI mutations return the new canonical snapshot atomically.

### Restricted JSON Patch

`deck.patch`, `design.patch`, and `slide.spec.patch` use RFC 6902 semantics restricted to `add`, `remove`, and `replace`. `add` and `replace` require an explicit `value` (including explicit JSON `null`); `remove` forbids `value`. Array append (`/-`), array indexes, JSON Pointer escaping, ordered application, and all-or-nothing failure follow the standard implementation rather than project-specific traversal code.

Each mutation operation has one shared path-policy definition used by both the model-facing Tool Schema and server-side enforcement. Runtime-owned identity, revision, project, and timestamp paths are never patchable. A patch is committed only after every operation succeeds and the resulting document passes its complete domain validation.

## Process restart and Run lifecycle

`pending`, `running`, `waiting`, `paused`, and `recovering` are non-terminal states. A normal server shutdown durably changes every in-memory Run owned by that process to `paused` before canceling its execution context. On startup, before the HTTP server is reachable, every persisted `pending`, `running`, `waiting`, or `recovering` Run is also reconciled to `paused`; no Run resumes automatically.

A paused Run remains the project's active writer and blocks new Runs and direct mutations. The user must explicitly resume or cancel it. Resume atomically claims `paused -> recovering` for the current process, restores the event cursor and latest checkpoint when present, reconciles tentative writes, and then returns to normal execution. A Run paused before its first checkpoint restarts safely from its original command. Process ownership guards graceful shutdown updates, and the conditional resume claim prevents two processes from recovering the same Run.

The frontend always reconciles a session against the backend's active-Run query, not only browser session storage or event history. This allows a reopened frontend to display a paused Run even when the previous process ended before emitting its first event.

## Runtime tool contract

Runtime owns the only executable tool registry. A domain tool is disclosed to the model only if that same registry permits it to execute for the current mode, phase, scope, capability and risk policy; exposure and execution must never use separate authorization rules.
The execution input must carry the identical mode and scope from its `RunCommand`; Runtime rejects any drift instead of authorizing from a second copy of that state.

| Tool | Capability / risk | Availability |
| --- | --- | --- |
| `read_ppt` | `ppt.read` / low, read-only | All Runtime modes in their supported read phases; resource schema is scope-filtered. |
| `render_slide` | `ppt.render` / low, read-only | Execute mode for `ppt` scopes only; slide schema is scope-filtered. |
| `mutate_ppt` | `ppt.mutate` / medium, write | Execute mode, executing phase, and a `spec` or `ppt` scope only; operation schema is scope-filtered. |

Runtime control actions (`create_plan`, `update_plan`, `ask_user`, `review_completion`, `finish`) are separately disclosed by the current phase and plan state. They must be the sole call in a model response.

The model receives only names, descriptions and parameter schemas. Capability labels and risk levels are Runtime-internal and must not be duplicated in prompts as a second authorization system. If an already disclosed domain tool is ever denied by Runtime policy, Runtime terminates the run immediately because another model turn cannot repair a server-policy inconsistency.

Context retrieval is Runtime-internal context engineering, not a model-visible tool. Runtime selects authorized, in-scope context and injects useful summaries into the dynamic task state; the model uses `read_ppt` when exact PPT resource content is required.

## Runtime presentation

Page ordinal, total, section ancestry, numbering visibility, and shared chrome are rebuilt from the current deck, outline, and design for preview, thumbnails, render evidence, and downstream export presentation. Reordering therefore changes the Runtime frame immediately without rewriting the slide spec or HTML body. Hidden page numbers still consume their outline ordinal.

## Frontend authority

The project store keeps one content snapshot. Every directory row, overview item, adjacent-page action, pending skeleton, and visual page number is derived from its outline tree. Selection persists only `currentSlideId`; page index is always derived.
