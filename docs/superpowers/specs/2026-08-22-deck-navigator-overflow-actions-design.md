# Deck Navigator Overflow Actions

## Scope

This update refines the left-hand deck navigator in `frontend/src/features/deck/DeckNavigator.tsx`. It applies only to the presentation-authoring navigator and does not change slide, outline, or persistence contracts.

## Decisions

- A section stays a single unobtrusive row; expansion remains its primary action.
- On row hover or keyboard focus, the visible section number yields to the same directional triangle used for expand/collapse, while a right-aligned overflow trigger appears.
- The section overflow menu contains `新增子节` and the destructive `删除章节` action. Existing confirmation remains required for deletion.
- A page row exposes one overflow trigger instead of resident vertical controls. Its menu contains `上移本页`, `下移本页`, and destructive `删除本页`; drag-and-drop reordering remains available.
- Page numbers are 16px, bold, and dark neutral, so they are visually distinct from section and subsection numbering.
- The footer label changes from `加页` to `新增页面`.

## Non-goals

- No change to how sections, subsections, pages, or ordering APIs are stored or validated.
- No changes to the existing subsection deletion affordance.

## Acceptance criteria

1. Section and page actions are each accessed through a single three-dot menu.
2. Menu actions preserve existing mutation, disabled-state, and confirmation behavior.
3. Collapsing/expanding a section remains available from the full section row.
4. Page numbers are clearly more prominent than outline numbering.
5. Navigator tests cover the new menus, page ordering, deletion, and footer label.
