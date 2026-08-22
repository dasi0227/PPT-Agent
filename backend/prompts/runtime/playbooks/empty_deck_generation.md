Task playbook: empty deck generation.

Use this playbook when generating a complete presentation from an empty or effectively empty deck.

Canonical order:
1. Establish deck:outline with clear audience, goal, positioning, requirements, prohibitions, section purposes, RunCommand.options.language when supplied, and a slide order inside RunCommand.options.range when supplied.
2. Establish deck:design with theme, direction, density and shared chrome.
3. Create each slide:<id>:spec in outline order with role, title, key message, semantic elements and optional layout direction.
4. Create each slide:<id>:html as the final implementation using project CSS, semantic HTML and local/data/blob-safe resources.
5. Render every generated HTML slide.
6. Repair blocking render diagnostics and render affected slides again.

Structure rule (strict two levels):
- Each section is exactly one form: direct (no subsections; its pages set only section_id) or grouped (one or more subsections; every page sets a subsection_id owned by that section).
- Never mix direct pages and subsections under the same section. Choose per section: leave subsections empty, or route all its pages through subsections.

Quality expectations:
- The deck should have one coherent narrative, not isolated pages.
- Visual language should be consistent across all slides.
- Information density should fit a live presentation.
- Each page should show deliberate composition, hierarchy and whitespace.

Completion:
- Do not finish until all generated HTML pages have required evidence.
- The final message should summarize deck structure, design direction, generated pages and checks performed.
