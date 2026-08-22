# Deck Navigator Overflow Actions

## Scope

This update refines the left-hand deck navigator and its manual title-editing contract. It covers the presentation-authoring navigator, the directory mutation API, and file-owned outline/slide specs.

## Decisions

- A section stays a single unobtrusive row; expansion remains its primary action.
- On row hover or keyboard focus, the visible section number yields to the same directional triangle used for expand/collapse, while a right-aligned overflow trigger appears.
- The section overflow menu contains `重命名`, `新增子节`, and the destructive `删除章节` action. Existing confirmation remains required for deletion.
- A subsection uses the same hover-only overflow trigger instead of a resident delete icon. Its menu contains `重命名` and destructive `删除子节`.
- A page row exposes one overflow trigger instead of resident vertical controls. Its menu contains `重命名`, `上移本页`, `下移本页`, and destructive `删除本页`; drag-and-drop reordering remains available.
- Selecting `重命名` opens the same lightweight form-modal pattern used by projects and sessions. Names are trimmed, must contain 1–60 Unicode characters, and are disabled while an Agent run is active.
- Section and subsection renames update `outline.json`; page renames update the page `spec.json`. Every rename endpoint returns the authoritative `{slides, spec}` snapshot, which the frontend applies atomically.
- The directory exposes dedicated rename endpoints:
  - `PATCH /projects/:id/sections/:section_id`
  - `PATCH /projects/:id/sections/:section_id/subsections/:subsection_id`
  - `PATCH /projects/:id/slides/:slide_id`
- Hover state belongs to the exact row being pointed at. Descendant page hover must not activate its parent section controls.
- Section, subsection, and page overflow triggers have no persistent white backing plate; only the shared icon-button hover feedback may add a temporary neutral tint.
- Page numbers are 18px, bold, and dark neutral, so they are visually distinct from section and subsection numbering.
- Page titles are 16px and semibold, remain single-line, and truncate only when the navigator width cannot contain the full title.
- The footer label changes from `加页` to `新增页面`.

## Non-goals

- Renaming does not alter IDs, page order, placements, section purpose, slide role, key message, or rendered HTML.
- No inline contenteditable behavior; all renames use the explicit overflow-menu command and modal.

## Acceptance criteria

1. Section, subsection, and page actions are each accessed through a single three-dot menu.
2. Each menu exposes `重命名`; successful edits persist after reload and update the directory from the returned authoritative snapshot.
3. Invalid titles and writes during an active run are rejected by both UI gating and backend validation.
4. Menu actions preserve existing ordering, disabled-state, and deletion confirmation behavior.
5. Collapsing/expanding a section remains available from the full section row, and only hovering that row changes its number into a disclosure icon.
6. Page numbers remain clearly more prominent than outline numbering.
7. Service, HTTP, and navigator tests cover title persistence, menu contents, snapshot application, hover scope, page ordering, deletion, and footer labels.
