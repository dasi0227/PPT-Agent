PPT business policy.

Resource ownership:
- deck:outline owns deck goal, audience, narrative sections, subsection structure and stable slide order.
- deck:design owns the deck-wide 16:9 visual system: canvas, palette, typography, spacing, grid, density, components and motion direction.
- slide:<slide_id>:spec owns one page's semantic role, title, key message, content hierarchy, visual intent, asset intent and speaker notes.
- slide:<slide_id>:html is the final page implementation.

Presentation implementation rules:
- Every Slide HTML must use a 1600x900 .slide-stage.
- Every Slide HTML must link ../../common/tokens.css and ../../common/base.css.
- Runtime derives tokens.css from Design.
- Use accessible semantic HTML, useful alt text, CJK-safe fonts and readable typography.
- Use project-local, data or blob resources only.
- Do not pass disk paths, project paths, runtime paths, database identifiers or storage artifact kinds as model-visible resources.

Available shared tokens:
--color-bg, --color-fg, --color-primary, --color-accent, --color-muted,
--font-sans, --font-serif, --font-mono,
--text-title, --text-h1, --text-body, --text-caption,
--space-1, --space-2, --space-3, --space-4, --space-6, --space-8,
--radius-sm, --radius-md, --radius-lg,
--shadow-card, --shadow-pop, --stage-w and --stage-h.

Tool selection:
- Use read_ppt when exact saved text is needed.
- Use edit_ppt only for small uniquely anchored replacements.
- Use write_ppt for full creation, broad reconstruction or schema-normalized JSON model updates.
- After every latest HTML or design-affecting change, render affected pages, observe diagnostics, repair failures in the same ReAct loop, and render again.
