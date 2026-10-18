---
id: core.contracts
description: Specify the contracts of the html.
---
## Public Classes

### Canvas and Text

| Class | Purpose |
| --- | --- |
| `.slide-stage` | Fixed canvas and root of inherited content colors. |
| `.slide-content` | Optional full-canvas layer with theme-defined content padding. |
| `.slide-title` | Main heading. |
| `.section-title` | Heading within the slide. |
| `.slide-subtitle` | Lead text. |
| `.slide-body` | Body text. |
| `.caption` | Notes, sources, and supporting text. |
| `.kicker` | Short introductory label using the technical font. |
| `.text-emphasis` | Inline emphasis using contextual color and weight. |
| `.text-muted` | Secondary text using contextual color. |
| `.text-highlight` | Inline highlight with paired background and foreground. |

### Surfaces and States

| Class | Purpose |
| --- | --- |
| `.card` | Padded surface with border, radius, shadow, and matching content colors. |
| `.card-soft` | Modifier for `.card`: soft surface without visible border or shadow. |
| `.card-outline` | Modifier for `.card`: transparent surface with an outline and inherited content colors. |
| `.card-accent` | Modifier for `.card`: accent surface with matching content colors. |
| `.callout` | Standalone padded information block with surface and border. |
| `.status` | Compact inline status label. |
| `.tone-success`, `.tone-warning`, `.tone-danger`, `.tone-info` | Contextual colors for favorable, cautionary, unfavorable, and informational states. |

### Content and Images

| Class | Purpose |
| --- | --- |
| `.metric` | Vertical group containing a value, label, and optional change indicator. |
| `.metric-value` | Prominent numeric value with tabular figures; also usable independently. |
| `.metric-label` | Supporting metric label. |
| `.metric-unit` | Smaller secondary unit within or beside a value. |
| `.metric-change` | Compact change indicator; supports a tone. |
| `.quote` | Display quotation with an emphasis rule. |
| `.slide-list` | Apply to `ul` or `ol` for themed text, markers, and spacing; supports nested lists. |
| `.image-frame` | Clipped image container with themed background, border, and radius. A direct image child defaults to full width and natural aspect ratio. |
| `.image-cover` | Apply to an image to fill a sized container, preserving aspect ratio and cropping excess. |
| `.image-contain` | Apply to an image to fit the whole image inside a sized container, preserving aspect ratio. |
| `.pill` | Compact outlined tag. |
| `.divider` | Themed separator. |

### Tables

| Class or selector | Purpose |
| --- | --- |
| `.data-table` | Apply to `table` for full width, cell padding, aligned headers, and row separators. |
| `.table-numeric` | Apply to each numeric `th` and `td` within `.data-table` for end alignment and tabular figures. |
| `.table-focus` | Apply to a `tr`, `th`, or `td` within `.data-table` to highlight the row or cell. |
| `.table-striped` | Modifier for `.data-table`: alternating body-row backgrounds. |
| `.table-bordered` | Modifier for `.data-table`: borders around every cell; may combine with `.table-striped`. |
| `.data-table > tfoot` | Receives summary styling automatically; no additional class is needed. |

### Charts and Diagrams

| Class | Purpose |
| --- | --- |
| `.chart-primary`, `.chart-secondary`, `.chart-accent` | Primary, secondary, and accent data colors. |
| `.chart-series-1` through `.chart-series-8` | Discrete category colors. |
| `.chart-highlight`, `.chart-muted` | Emphasized and contextual data colors. |
| `.chart-positive`, `.chart-negative` | Numerically positive and negative values; sign does not imply a favorable or unfavorable outcome. |
| `.chart-line` | Unfilled SVG data line using the inherited series color. |
| `.chart-axis`, `.chart-grid` | Unfilled SVG axes and reference lines using contextual colors. |
| `.chart-label`, `.chart-value` | SVG chart labels and emphasized numeric values. |
| `.chart-legend` | Apply to `ul` for a wrapping legend with aligned direct `li` children. |
| `.chart-legend-key` | HTML color key; combine with the corresponding chart color class. |
| `.diagram-node` | SVG node shape with themed surface and outline. |
| `.diagram-edge` | Unfilled SVG connector; arrowheads require an explicit SVG marker. |
| `.diagram-label` | SVG text with foreground colors matching `.diagram-node`. |

## Public CSS Variables

### Colors

| Variables | Meaning |
| --- | --- |
| `--color-bg` | Slide background. |
| `--color-fg`, `--color-secondary`, `--color-caption` | Main, secondary, and supporting text on the slide background. |
| `--color-primary` | Primary text emphasis, rules, and outlines. |
| `--color-accent` | Accent surface background. |
| `--color-on-accent`, `--color-on-accent-secondary`, `--color-on-accent-caption` | Main, secondary, and supporting foregrounds on the accent surface. |
| `--color-surface` | Standard surface background. |
| `--color-on-surface`, `--color-on-surface-secondary`, `--color-on-surface-caption` | Main, secondary, and supporting foregrounds on the standard surface. |
| `--color-surface-soft` | Soft surface background. |
| `--color-on-surface-soft`, `--color-on-surface-soft-secondary`, `--color-on-surface-soft-caption` | Main, secondary, and supporting foregrounds on the soft surface. |
| `--color-border` | Default borders and separators. |
| `--color-highlight`, `--color-on-highlight` | Paired highlight background and foreground. |

### Typography

| Variables | Meaning |
| --- | --- |
| `--font-sans`, `--font-display`, `--font-mono` | Body, display, and technical font families. |
| `--text-title`, `--text-section`, `--text-subtitle`, `--text-body`, `--text-caption`, `--text-metric` | Font sizes for the corresponding roles. |
| `--weight-body`, `--weight-title`, `--weight-emphasis` | Body, heading, and emphasis weights. |
| `--leading-title`, `--leading-body`, `--leading-caption` | Heading, body, and supporting-text line heights. |
| `--tracking-title` | Heading letter spacing. |

### Spacing and Surfaces

| Variables | Meaning |
| --- | --- |
| `--space-1`, `--space-2`, `--space-3`, `--space-4`, `--space-6`, `--space-8` | General spacing steps, ordered from smallest to largest. |
| `--stage-padding-x`, `--stage-padding-y` | Horizontal and vertical content insets. |
| `--card-padding` | Card and callout padding. |
| `--table-padding-x`, `--table-padding-y` | Horizontal and vertical cell padding. |
| `--list-gap`, `--metric-gap`, `--quote-padding` | List-item spacing, metric-group spacing, and quotation rule-to-text spacing. |
| `--border-width`, `--border-style`, `--quote-border-width` | Standard border width and style, and quotation rule width. |
| `--radius-sm`, `--radius-md`, `--radius-lg` | Small, medium, and large corner radii. |
| `--shadow-sm`, `--shadow-card`, `--shadow-lg` | Subtle, card, and elevated shadows. |

### Charts

| Variables                                                | Meaning                                                        |
| -------------------------------------------------------- | -------------------------------------------------------------- |
| `--chart-primary`, `--chart-secondary`, `--chart-accent` | Primary, secondary, and accent data colors.                    |
| `--chart-series-1` through `--chart-series-8`            | Discrete category colors.                                      |
| `--chart-highlight`, `--chart-muted`                     | Emphasized and contextual data colors.                         |
| `--chart-positive`, `--chart-negative`                   | Colors for numerically positive and negative values.           |
| `--chart-line-width`                                     | Data-line stroke width as a unitless SVG coordinate value.     |
| `--chart-grid-width`                                     | Grid and axis stroke width as a unitless SVG coordinate value. |

### Local Context

| Variable | Meaning |
| --- | --- |
| `--content-bg` | Current container background. |
| `--content-fg` | Main foreground. |
| `--content-secondary` | Secondary foreground. |
| `--content-caption` | Supporting foreground. |
| `--content-emphasis` | Emphasis color. |
| `--content-border` | Border color. |
| `--chart-series` | Current chart color for data lines and legend keys. |
