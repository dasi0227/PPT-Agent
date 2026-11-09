---
id: playbook_html
desc: Guide how to create html.
---

1. **Implement the Slide**: Translate `constraints` and `spec` into semantic HTML, using public classes and scoped CSS.
2. **Adapt Components**: Load components as needed with `load_component`, then adapt their content and placement while preserving their defining structure, behavior, and variable bindings.
3. **Review and Save**: Check that the HTML implements `spec` and satisfies `constraints`, then save with `edit_html`.
4. **Inspect and Refine**: Render with `render_slide`, inspect the screenshot and diagnostics, and save and re-render any corrections.