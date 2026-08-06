Task playbook: empty deck generation.

Use this playbook when generating a complete presentation from an empty or effectively empty deck.

Canonical order:
1. Establish deck:outline with clear audience, goal, narrative arc, sections and stable slide order.
2. Establish deck:design with a coherent 16:9 visual system, palette, typography, spacing, grid, density and motion direction.
3. Create each slide:<id>:spec in outline order with role, title, key message, hierarchy, visual intent and speaker notes.
4. Create each slide:<id>:html as the final implementation using project CSS, semantic HTML and local/data/blob-safe resources.
5. Render every generated HTML slide.
6. Repair blocking render diagnostics and render affected slides again.

Quality expectations:
- The deck should have one coherent narrative, not isolated pages.
- Visual language should be consistent across all slides.
- Information density should fit a live presentation.
- Each page should show deliberate composition, hierarchy and whitespace.

Completion:
- Do not finish until all generated HTML pages have required evidence.
- The final message should summarize deck structure, design direction, generated pages and checks performed.
