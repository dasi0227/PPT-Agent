# Agent Interaction Polish Design

> Supersession note: section 3's frontend-only `Enhance` preview is superseded by
> `2026-08-23-contextual-prompt-polish-design.md`. The remaining interaction
> improvements in this document are unchanged.

## Scope

This change improves five existing agent-facing interactions without changing the public event protocol, Run lifecycle, Plan schema, Question schema, or backend APIs:

- replace the live progress spinner with a locally adapted AICSS B2 Orb;
- improve the existing Plan dropdown with todo-style status markers and animated completion counts;
- add a one-second frontend-only Enhance affordance to the composer;
- give grouped Questions explicit manual slide transitions;
- collapse pending Plan Approval Markdown into an expandable preview.

The work preserves the current three-pane PowerPoint workspace, compact right-panel density, Tailwind design tokens, keyboard behavior, history replay, and reduced-motion support.

## Design Direction

The product is a PowerPoint creation workspace for users supervising an Agent. The right panel's job is to make progress, input, and blocking decisions legible without competing with the slide canvas.

The existing neutral Office-like palette and typography remain authoritative:

- `panel` / `surface`: quiet working planes;
- `text-900` / `text-600` / `text-400`: primary, supporting, and metadata hierarchy;
- `accent` / `accent-soft`: active Agent state;
- `success`: completed work;
- `danger`: failed or destructive state.

No new font family or global color palette is introduced. The signature visual is the B2 Orb beside live progress text. Motion elsewhere remains brief and functional: rolling counts, question navigation, the Enhance sweep, and Plan disclosure.

## 1. Live Progress B2 Orb

`LiveProgressRow` replaces its generic `Loader2` with a local B2 Orb implementation derived from the MIT-licensed AICSS geometry and animation.

- Only the live progress row changes; running tool rows keep their current loaders.
- The Orb is 18 px and inherits the current accent color.
- One B2 variant is shipped; the full AICSS component library is not added.
- Progress copy, optional `current / total`, and `aria-live=polite` remain unchanged.
- Under `prefers-reduced-motion`, the Orb shows a stable recognizable frame.

## 2. Plan Dropdown Todo States

`PlanIndicator` keeps its current dropdown, title, always-visible step list, maximum height, and trigger badge. It does not gain a collapsible todo header.

Step markers become:

- pending: dashed circle;
- in progress: right arrow inside a circle;
- completed: check inside a circle;
- failed: existing danger-colored failure marker.

The active row keeps a restrained accent-soft background. Both the trigger badge and dropdown header show `completed / total`. Changed count glyphs roll vertically for roughly 300 ms and render without animation when reduced motion is requested.

Plan data continues to derive exclusively from `PlanState.steps`; no protocol or store change is required.

## 3. Frontend-only Enhance

The composer adds a `Sparkles` button inside the text area's upper-right corner.

Visibility and behavior:

- visible only when editable text is non-empty;
- available for both new-run instructions and running-run steering text;
- absent or disabled whenever the composer itself is disabled;
- reserves text-area padding so text does not pass under the control.

Clicking Enhance starts a one-second local phase:

1. retain the exact input value and selection;
2. mark the composer read-only and disable Enhance and Send;
3. animate the Sparkles control and a soft horizontal sweep over the text area;
4. announce `正在优化表达` through a polite live region;
5. restore focus and selection when the phase ends;
6. leave the text byte-for-byte unchanged.

No request is made and no success message claims that content changed. Reduced motion removes the sweep while keeping the one-second busy state. The future backend can replace the timer with an enhancer function without changing the visible state model.

## 4. Manual Grouped Question Slides

Question protocol, drafts, custom answers, submission, and answered-history rendering remain unchanged.

The pending grouped-question body becomes an overflow-hidden viewport whose track translates horizontally:

- next moves the current question left and the next question in from the right;
- previous uses the reverse direction;
- navigation occurs only from explicit previous/next controls;
- selecting an option never advances automatically;
- users may visit later questions before answering the current one;
- every draft remains editable until submission;
- the primary `继续` action is disabled until all questions are answered;
- submitting changes its label to `提交中`;
- reduced motion swaps questions immediately.

The viewport height follows the active question with a short transition instead of reserving a fixed 330 px card body.

## 5. Plan Approval Preview

Only a pending Plan Approval collapses its flexible Markdown `plan.content`. The title, step count, status, decision controls, revision feedback, and submit action remain visible.

- Default content height is 480 px, leaving enough room for a substantial portion of the plan before the disclosure.
- Overflowing content ends in a surface-colored gradient with a restrained blur.
- A centered `展开全部` control sits above the fade.
- Short content renders normally without a fade or disclosure control.
- Expanded content renders completely without a nested scroll region and offers a quiet `收起` action.
- Disclosure does not reset the selected decision or revision feedback.
- Approval does not require opening the full content.

The already-answered approval card keeps its existing outer collapsed summary. When the user expands that historical summary, its full Plan content appears directly; no nested disclosure is added.

## Non-goals

- installing `@aicss/react` or adopting unrelated AICSS components;
- changing tool activity rows;
- changing SSE events, Run state, Plan data, or Question data;
- implementing real prompt enhancement, before/after comparison, undo, or enhancement history;
- automatic question advance;
- automatic Plan approval;
- a collapsible header inside the Plan progress dropdown.

## Accessibility

- Every new icon-only control has an accessible name and visible keyboard focus.
- Busy and progress text remains available to assistive technology.
- Question navigation and Plan disclosure use native buttons and correct disabled state.
- All new motion has a reduced-motion path.
- Enhance restores keyboard focus and text selection after its temporary busy state.

## Verification

Frontend tests cover:

- B2 Orb rendering and progress count preservation;
- todo marker states and updated completion count;
- Enhance visibility in idle and steering contexts, one-second unchanged text, disabled send, and focus restoration;
- manual-only Question navigation, draft persistence, reverse navigation, and all-answered gating;
- Plan preview overflow disclosure, short-content behavior, and approval-state preservation.

Run formatting/type checks, the full frontend test suite, and a production build. Perform a visual pass at normal and narrow right-panel widths and with reduced motion enabled.

## Acceptance Criteria

1. Live Agent progress uses B2 Orb while tool activity visuals remain unchanged.
2. The Plan dropdown communicates pending, active, completed, and failed steps without adding a collapsible header.
3. Enhance appears for both new requests and steering text, runs for one second, and does not change text or call a backend.
4. Questions move only on explicit navigation and cannot continue until every question is answered.
5. Pending Plan Approval initially occupies a compact height and exposes complete Markdown through `展开全部`.
6. Existing Run, Plan, Question, history, and submission behavior continues to pass tests.
