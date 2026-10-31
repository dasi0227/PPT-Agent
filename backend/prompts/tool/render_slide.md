---
id: tool_render_slide
desc: Render a slide and optionally request a quality assessment.
---

Render a slide’s saved HTML into a screenshot and report rendering problems. Set `assess_quality` to `true` to request an independent assessment of content coverage, expression clarity and compliance with applicable requirements. A successful call returns the rendered screenshot and `diagnostics` describing rendering problems. When requested and available, `preview` provides advisory quality feedback with scores ranging from 0 to 3; higher is better.
