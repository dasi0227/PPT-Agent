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

## Runtime presentation

Page ordinal, total, section ancestry, numbering visibility, and shared chrome are rebuilt from the current deck, outline, and design for preview, thumbnails, render evidence, and downstream export presentation. Reordering therefore changes the Runtime frame immediately without rewriting the slide spec or HTML body. Hidden page numbers still consume their outline ordinal.

## Frontend authority

The project store keeps one content snapshot. Every directory row, overview item, adjacent-page action, pending skeleton, and visual page number is derived from its outline tree. Selection persists only `currentSlideId`; page index is always derived.
