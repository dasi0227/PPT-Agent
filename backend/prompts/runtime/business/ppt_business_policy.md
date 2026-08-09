PPT business policy.

Resource ownership:
- deck:outline owns deck goal, audience, narrative sections, subsection structure and stable slide order.
- deck:design owns theme selection, deck direction, density and shared chrome. The 1600x900 canvas is a Runtime protocol.
- slide:<slide_id>:spec owns one page's semantic role, title, key message, ordered element intents and optional layout direction.
- slide:<slide_id>:html is the final page implementation.
These names are display keys only. Tool calls must use resource objects such as {"type":"deck","part":"outline"} or {"type":"slide","slide_id":"slide-01","part":"html"}, never display-key strings.

Presentation implementation rules:
- Every Slide HTML must use a 1600x900 .slide-stage.
- Every Slide HTML must link ../../common/tokens.css and ../../common/base.css.
- Runtime derives tokens.css from Design.
- Use accessible semantic HTML, useful alt text, CJK-safe fonts and readable typography.
- Use project-local, data or blob resources only.
- Do not pass disk paths, project paths, runtime paths, database identifiers, storage artifact kinds, display keys or "current" as tool resource arguments.

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
