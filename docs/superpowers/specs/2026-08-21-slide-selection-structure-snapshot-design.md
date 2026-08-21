# Slide ID Selection and Structure Snapshot Design

## Scope

This change replaces index-based page selection and split slide/spec refreshes in the workspace. It covers page selection, URL synchronization, structure mutations, section/subsection placement, and the frontend stores that expose project content.

The primary failure being fixed is the navigation feedback loop seen when a selected slide moves between adjacent subsection positions, such as moving a page from 3.2 to 3.1.

## Root Cause

The current workspace represents one selection in three incompatible forms:

- `deckStore.currentPage` stores a mutable array index.
- `projectStore.slidesByProjectId` defines the meaning of that index.
- the workspace URL stores a stable slide ID and synchronizes it bidirectionally with the index.

Slide order and slide placement are also refreshed through separate stores and requests. After a restructure, the new slide array can render while `currentPage`, the URL, or the spec view still represents the old structure. The inbound and outbound URL effects then correct each other repeatedly.

## Key Decisions

### Stable page identity

`currentSlideId` is the only stored page selection. Numeric page indices are derived from the current ordered slide snapshot and are never persisted in a store or URL.

Moving a slide does not change its identity. A reorder therefore leaves `currentSlideId` and the URL unchanged.

### Unified project content state

Slides and `SpecProjectView` belong to one project-content snapshot and are updated by one Zustand store action. The standalone spec store is removed. Consumers cannot observe a new slide order paired with an old outline or placement map.

Loading project content fetches slides and spec concurrently, then publishes both only after both requests succeed.

### Authoritative structure mutation response

The backend restructure operation returns HTTP 200 with a canonical structure snapshot:

```json
{
  "slides": [],
  "spec": {
    "outline": {},
    "slide_specs": {},
    "design": {},
    "materialization": {}
  }
}
```

The snapshot is read after the mutation succeeds. The frontend applies this response directly and does not issue separate slide/spec refresh requests.

### URL synchronization

The `slide` query parameter stores `currentSlideId` directly.

- Initial load or browser navigation hydrates the store from a valid URL slide ID.
- User selection updates `currentSlideId`, then mirrors that ID into the URL.
- Structure changes do not update the URL because the selected ID does not change.
- If the selected slide is deleted, the client selects the next slide when possible, otherwise the previous slide, otherwise no slide.

### Structure mutation behavior

Up, down, and drag operations submit stable slide IDs and placements. While one mutation is pending, further structure mutations are disabled. A failed request leaves the existing snapshot and selection untouched and shows the existing inline error treatment.

## Backend Changes

- Change `SlideService.RestructureSlides` to return a structure snapshot after validation and persistence.
- Return the snapshot from `POST /projects/:id/slides/restructure` with HTTP 200.
- Keep validation, active-run conflict handling, and persistence rules unchanged.
- Add handler and service tests for canonical order, placement, revisions, and response shape.

## Frontend Changes

- Replace `currentPage`, `setCurrentPage`, `goNext`, and `goPrev` with `currentSlideId` and `setCurrentSlideId`.
- Derive indices locally wherever page numbers or adjacent navigation are needed.
- Absorb spec state and actions into `projectStore`, exposing atomic project-content snapshots.
- Remove `specStore` and migrate its consumers.
- Change the restructure API client to return a structure snapshot.
- Apply restructure responses directly without refetching.
- Rewrite URL state synchronization around slide IDs.

## Error and Empty-State Behavior

- Project-content loading publishes no partial snapshot when either slides or spec fails.
- A restructure failure keeps the previous snapshot and selected slide ID.
- A missing or invalid URL slide ID selects the first available slide and normalizes the URL.
- An empty project has `currentSlideId = null` and no `slide` query parameter.

## Non-goals

- No compatibility layer for `currentPage` or the old 204 restructure response.
- No migration for persisted development data is required.
- No change to slide rendering, message/run protocols, or the visual directory design.

## Acceptance Criteria

- Moving the selected page from subsection 3.2 to 3.1 produces no intermediate adjacent-page selection.
- The selected slide ID and URL remain constant through up, down, and drag reorder operations.
- Slides, outline order, and section/subsection placements update together from one response.
- Previous/next navigation and overview selection continue to work.
- Direct links and browser back/forward select the intended slide by ID.
- Deleting the selected slide chooses a deterministic adjacent fallback.
- Frontend unit/integration tests, backend tests, lint, type checking, and production build pass.
