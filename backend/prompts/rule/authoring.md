---
id: rule.authoring
description: Formulate the rules for authoring slide's HTML.
---
- **Document**: Create one complete HTML document per slide. Include exactly one `.slide-stage` fixed at 1920 × 1080 CSS pixels. Keep custom CSS, SVG, and scripts within the document. MUST NOT add a responsive canvas shell, viewport scaling, or presentation navigation.
- **Runtime**: Runtime supplies fonts, public styles, canvas scaling, centering, and shared decorations (page numbers, presentation titles, section titles, and the logo). MUST NOT duplicate these features.
- **Styling**: Scope custom selectors to the slide or component. Preserve variable references instead of copying resolved values. MUST NOT redefine public classes or global theme variables.
- **Assets**: Use supplied project asset paths and inline SVG or data for authored graphics. MUST NOT depend on remote fonts, stylesheets, scripts, network requests, or temporary blob URLs.
- **Output**: Essential content must appear on initial load and remain complete and static. MUST NOT require clicks, hover, scrolling, or unfinished animation to reveal it.