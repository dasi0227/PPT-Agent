---
id: core_contracts
desc: Specify the contracts of the html.
---

## Public Classes

### Canvas and Text

- `.slide-stage`: Fixed canvas and root of inherited content colors.
- `.slide-content`: Optional full-canvas layer with theme padding.
- `.slide-title`: Main heading.
- `.section-title`: Heading within the slide.
- `.slide-subtitle`: Lead text.
- `.slide-body`: Body text.
- `.caption`: Notes, sources, and supporting text.
- `.kicker`: Short introductory label in the technical font.
- `.text-emphasis`: Inline emphasis with contextual color and weight.
- `.text-muted`: Secondary text in contextual color.
- `.text-highlight`: Inline highlight with paired background and foreground.

### Surfaces and States

- `.card`: Padded surface with border, radius, shadow, and matching content colors.
- `.card-soft`: `.card` modifier: soft surface without visible border or shadow.
- `.card-outline`: `.card` modifier: transparent outlined surface with inherited content colors.
- `.card-accent`: `.card` modifier: accent surface with matching content colors.
- `.callout`: Standalone padded block with background and border.
- `.status`: Compact inline status label.
- `.tone-success`, `.tone-warning`, `.tone-danger`, `.tone-info`: Contextual colors for favorable, cautionary, unfavorable, and informational states.

### Content and Images

- `.metric`: Vertical group of value, label, and optional change indicator.
- `.metric-value`: Prominent tabular figures; usable without `.metric`.
- `.metric-label`: Supporting metric label.
- `.metric-unit`: Smaller secondary unit within or beside a value.
- `.metric-change`: Compact change indicator; supports a tone.
- `.quote`: Display quotation with an emphasis rule.
- `.slide-list`: Themed text, markers, and spacing for `ul` or `ol`; supports nesting.
- `.image-frame`: Clipped container with themed background, border, and radius. Direct `img` children default to full width and natural aspect ratio.
- `.image-cover`: On an image: fills a sized container, preserving aspect ratio and cropping excess.
- `.image-contain`: On an image: fits the whole image inside a sized container, preserving aspect ratio.
- `.pill`: Compact outlined tag.
- `.divider`: Themed separator.

### Tables

- `.data-table`: Full-width `table` with padded cells, aligned headers, and row separators.
- `.table-numeric`: End alignment and tabular figures on each numeric `th` or `td` in `.data-table`.
- `.table-focus`: Highlights a `tr`, `th`, or `td` in `.data-table`.
- `.table-striped`: `.data-table` modifier: alternating body-row backgrounds.
- `.table-bordered`: `.data-table` modifier: borders on every cell; combines with `.table-striped`.
- `.data-table > tfoot`: Automatic summary styling.

### Charts and Diagrams

- `.chart-primary`, `.chart-secondary`, `.chart-accent`: Primary, secondary, and accent data colors.
- `.chart-series-1` through `.chart-series-8`: Discrete category colors.
- `.chart-highlight`, `.chart-muted`: Emphasized and contextual data colors.
- `.chart-positive`, `.chart-negative`: Numerically positive and negative values; sign does not imply a favorable or unfavorable outcome.
- `.chart-line`: Unfilled SVG data line in the current series color.
- `.chart-axis`, `.chart-grid`: Unfilled SVG axes and grid lines in contextual colors.
- `.chart-label`, `.chart-value`: SVG chart labels and emphasized numeric values.
- `.chart-legend`: Wrapping `ul` legend with aligned direct `li` children.
- `.chart-legend-key`: HTML color key; pair with the matching chart color class.
- `.diagram-node`: SVG node shape with themed surface and outline.
- `.diagram-edge`: Unfilled SVG connector; arrowheads require an SVG marker.
- `.diagram-label`: SVG text matching `.diagram-node` foreground colors.

## Public CSS Variables

### Colors

- `--color-bg`: Slide background.
- `--color-fg`, `--color-secondary`, `--color-caption`: Main, secondary, and supporting text on the slide background.
- `--color-primary`: Primary text emphasis, rules, and outlines.
- `--color-accent`: Accent surface background.
- `--color-on-accent`, `--color-on-accent-secondary`, `--color-on-accent-caption`: Main, secondary, and supporting foregrounds on accent surfaces.
- `--color-surface`: Standard surface background.
- `--color-on-surface`, `--color-on-surface-secondary`, `--color-on-surface-caption`: Main, secondary, and supporting foregrounds on standard surfaces.
- `--color-surface-soft`: Soft surface background.
- `--color-on-surface-soft`, `--color-on-surface-soft-secondary`, `--color-on-surface-soft-caption`: Main, secondary, and supporting foregrounds on soft surfaces.
- `--color-border`: Default borders and separators.
- `--color-highlight`, `--color-on-highlight`: Paired highlight background and foreground.

### Typography

- `--font-sans`, `--font-display`, `--font-mono`: Body, display, and technical fonts.
- `--text-title`, `--text-section`, `--text-subtitle`, `--text-body`, `--text-caption`, `--text-metric`: Font sizes by role.
- `--weight-body`, `--weight-title`, `--weight-emphasis`: Body, heading, and emphasis weights.
- `--leading-title`, `--leading-body`, `--leading-caption`: Heading, body, and supporting-text line heights.
- `--tracking-title`: Heading letter spacing.

### Spacing and Surfaces

- `--space-1`, `--space-2`, `--space-3`, `--space-4`, `--space-6`, `--space-8`: Spacing scale, smallest to largest.
- `--stage-padding-x`, `--stage-padding-y`: Horizontal and vertical content insets.
- `--card-padding`: Card and callout padding.
- `--table-padding-x`, `--table-padding-y`: Horizontal and vertical cell padding.
- `--list-gap`, `--metric-gap`, `--quote-padding`: List-item gaps, metric-group gaps, and quote border-to-text spacing.
- `--border-width`, `--border-style`, `--quote-border-width`: Standard border width and style, and quotation rule width.
- `--radius-sm`, `--radius-md`, `--radius-lg`: Small, medium, and large corner radii.
- `--shadow-sm`, `--shadow-card`, `--shadow-lg`: Subtle, card, and elevated shadows.

### Charts

- `--chart-primary`, `--chart-secondary`, `--chart-accent`: Primary, secondary, and accent data colors.
- `--chart-series-1` through `--chart-series-8`: Discrete category colors.
- `--chart-highlight`, `--chart-muted`: Emphasized and contextual data colors.
- `--chart-positive`, `--chart-negative`: Colors for positive and negative values.
- `--chart-line-width`: Data-line stroke width in unitless SVG coordinates.
- `--chart-grid-width`: Grid and axis stroke width in unitless SVG coordinates.

### Local Context

- `--content-bg`: Current container background.
- `--content-fg`: Main foreground.
- `--content-secondary`: Secondary foreground.
- `--content-caption`: Supporting foreground.
- `--content-emphasis`: Emphasis color.
- `--content-border`: Border color.
- `--chart-series`: Current series color for data lines and legend keys.
