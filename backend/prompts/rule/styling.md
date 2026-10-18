---
id: rule.styling
description: Formulate the rules for using public classes and theme variables.
---
- **Classes**: Use public classes for appearance and slide-specific CSS for composition. Apply required base, child, and modifier classes explicitly.
- **Surfaces**: Inherit `--content-*` within containers. Custom surfaces may override them locally and must apply matching backgrounds, foregrounds, and borders; variable assignments alone do not draw surfaces.
- **Cards and Tones**: Use at most one variant with `.card`. Apply at most one tone to `.status`, `.callout`, `.metric-change`, or a plain `.card`. Tones only set context variables.
- **Charts**: Color classes set SVG fill and `--chart-series`; `.chart-line` uses its own or inherited series color. Apply one color role per mark, group, or legend key, keeping category mappings consistent across charts and legends.