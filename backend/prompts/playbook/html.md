---
id: playbook.html
description: Guides slide HTML creation and revision using content plans, visual requirements, public classes, public CSS variables, and reusable components.
---

## Procedure

1. **Understand the Change**: Use the request and relevant references to identify what needs to be created, revised, or recomposed. Read missing requirements, current HTML, and visual evidence needed for the implementation.
2. **Build the Implementation**: Translate the content and composition into semantic HTML using public classes, public CSS variables, suitable components, and layout CSS scoped to the slide.
3. **Save the HTML**: Use `edit_html` to save the implementation. Choose local edits or complete replacement according to the change and the tool contract; use current saved source for exact anchors.
4. **Inspect and Refine**: Call `render_slide` and inspect the current screenshot and diagnostics against the references and quality requirements. When a previous version is supplied, compare the intended change, content preservation, and regressions before deciding to keep, adjust, or revert it. Use `diagnostics.text_clipping` to locate the affected text and clipping container; correct available space, wrapping, or layout while preserving required content. Render the affected slide again to verify the specific fix. Do not alternate between deleting clipped content and restoring it into the same insufficient space.

## References

- **Spec**: Use the slide's message, content elements, and relationships to determine what the slide presents and how its information is organized.
- **Design**: Use presentation-wide visual requirements and prohibitions to guide layout and styling.
- **Project Decorations**: Use the human-managed placements and fixed logo footprint in `project_decorations` to reserve space for enabled shared decorations.
- **Existing HTML**: Use current source to understand the existing structure, styles, and behavior and identify the changes needed.
- **Screenshots**: Images labeled `current` show the saved slide and its rendered dependencies. Images labeled `previous` show the preceding rendered version for comparison only and cannot prove current completion. Repeated renders of identical dependencies keep the same comparison baseline. After an edit and before a new render, only historical imagery may be available.
- **Components**: Use relevant component source as a reference for structure, scoped styles, exposed variables, and behavior, adapting it to the slide's content and composition.

## Rules「IMPORTANT」

- **Runtime**: Runtime isolates each slide, handles canvas scaling and centering, and loads fonts, public class definitions, and public CSS variables before slide-specific styles. It renders shared decorations at human-configured project positions, derives slide numbers from their order in the outline, and displays the uploaded logo in a fixed box while preserving its aspect ratio. On theme changes, it updates public styles and variables and dispatches `ppt:themechange`; scripts that cache computed styles must refresh those values and redraw on this event. Reserve space for shared decorations and do not duplicate them in authored HTML.
- **HTML Document**: Write a separate, complete HTML document for each slide, with `<!doctype html>`, a language attribute, UTF-8 metadata, and exactly one `.slide-stage` as its fixed 1920 × 1080 CSS-pixel canvas. Keep the slide's custom CSS, SVG, and scripts within that document. Do not combine multiple slides into one document or add viewport scaling, presentation navigation, or a responsive canvas shell.
- **Public Classes**: Use public classes for the roles and combinations defined in Contracts. Combine them with slide-specific classes for layout and necessary adjustments to size or spacing. Scope custom selectors to the slide or component, and reuse existing public styling rather than recreating it. Do not redefine public classes globally.
- **Public Variables**: Reference runtime-supplied global variables for custom styling without redefining them. Inside a container, use inherited `--content-*` variables for text, emphasis, and borders. Set context variables locally when establishing a custom surface, pairing its background with matching foregrounds and borders. Keep variable references through `var()`, `calc()`, or `color-mix()` instead of copying resolved values. Use fixed colors only when required by the content, such as brand marks or factual color encodings.
- **Component Adaptation**: Adapt a component's content, size, and placement through its exposed variables and scoped selectors while preserving its defining structure and behavior. Use component-specific class names and an additional instance class when an adjustment should affect only one instance. Preserve the component's existing variable bindings and fallbacks, including whether it inherits surrounding content colors or establishes its own surface. Do not replace these references with resolved values or add public classes indiscriminately to component internals.
- **Portable Assets**: Use available project assets through their supplied references, following the image reference rules. Use inline SVG or data for authored graphics. Do not make essential content depend on remote fonts, stylesheets, scripts, chart CDNs, network fetches, or temporary blob URLs.
- **Static Output**: Show essential content on initial load and in screenshots or PDF. Interaction and animation must leave a complete static and reduced-motion presentation; do not gate content behind clicks, hover, scrolling, or an unfinished animation. Do not assume slide-enter, leave, or replay APIs exist.

## Contracts「IMPORTANT」

### Public Classes

**Public classes supply appearance and a few content structures; slide-specific CSS supplies the composition. Modifiers apply only where described. Their names do not imply that other classes are automatically added to the element or its children.**

#### Canvas and text

- `.slide-stage`: The fixed slide canvas and root of inherited content colors; use exactly once.
- `.slide-content`: An optional full-canvas content layer with theme-defined safe padding; other authored elements may also live inside the stage.
- `.slide-title`: The slide's main heading.
- `.section-title`: A heading within the slide.
- `.slide-subtitle`: Lead text below or alongside a heading.
- `.slide-body`: Standard body text.
- `.caption`: Supporting text, notes, and source captions.
- `.kicker`: A short category or introductory label in the theme's technical font.
- `.text-emphasis`: Inline emphasis using the current container's emphasis color and weight.
- `.text-muted`: Secondary text using the current container's secondary color.
- `.text-highlight`: Inline highlighting with a paired background and foreground.

#### Surfaces and states

- `.card`: A padded surface with a border, radius, shadow, and matching inherited content colors.
- `.card-soft`: Combine with `.card` on the same element for a soft surface without a visible border or shadow.
- `.card-outline`: Combine with `.card` on the same element for a transparent outlined surface that inherits surrounding content colors.
- `.card-accent`: Combine with `.card` on the same element for an accent surface with matching primary, secondary, and caption text colors.
- `.callout`: A standalone padded information block with a surface and border.
- `.status`: A compact inline status label with a background, border, and text.
- `.tone-success`: Sets inherited colors for a successful or favorable state.
- `.tone-warning`: Sets inherited colors for a warning or caution.
- `.tone-danger`: Sets inherited colors for an error, risk, or unfavorable state.
- `.tone-info`: Sets inherited colors for an informational state.

> Use at most one card variant on a card. Apply one tone on the same element as `.status`, `.callout`, `.metric-change`, or a plain `.card`; tones set variables and do not draw a standalone container. Avoid combining a tone with a card variant.

#### Content and images

- `.metric`: A vertical group for a value, label, and optional change indicator.
- `.metric-value`: A prominent numeric value with tabular figures; usable inside or outside `.metric`.
- `.metric-label`: The supporting label for a metric.
- `.metric-unit`: A smaller secondary unit beside or within a metric value.
- `.metric-change`: A compact change indicator; combine with a tone when the meaning warrants it.
- `.quote`: A quotation with display typography and an emphasis rule.
- `.slide-list`: Apply to `ul` or `ol` for theme-sized text, markers, and item spacing; nested lists can use it too.
- `.image-frame`: A clipped image container with a themed background, border, and radius, typically a `figure` or `div`; a direct image child defaults to full width and its natural aspect ratio.
- `.image-cover`: Apply directly to an image to fill its sized container while preserving aspect ratio and cropping excess content.
- `.image-contain`: Apply directly to an image to fit the whole image inside its sized container while preserving aspect ratio.
- `.pill`: A compact category or tag label with an outline.
- `.divider`: A thematic separator, typically an `hr`.

#### Tables

- `.data-table`: Apply to `table` for full-width tabular content, cell padding, aligned headers, and row separators.
- `.table-numeric`: Apply to numeric `th` and `td` descendants of `.data-table` for end alignment and tabular figures; include the column header and each numeric cell.
- `.table-focus`: Apply to a `tr`, `th`, or `td` inside `.data-table` to highlight that row or cell with paired colors.
- `.table-striped`: Combine with `.data-table` on the same table for alternating body-row backgrounds.
- `.table-bordered`: Combine with `.data-table` on the same table for borders around every cell; may also be combined with `.table-striped`.
- `.data-table > tfoot`: A standard table footer receives summary styling and a stronger top separator automatically; no additional class is required.

#### Charts and diagrams

- `.chart-primary`: The theme's primary data fill.
- `.chart-secondary`: The theme's secondary data fill.
- `.chart-accent`: The theme's accent data fill.
- `.chart-series-1`: Discrete series color 1.
- `.chart-series-2`: Discrete series color 2.
- `.chart-series-3`: Discrete series color 3.
- `.chart-series-4`: Discrete series color 4.
- `.chart-series-5`: Discrete series color 5.
- `.chart-series-6`: Discrete series color 6.
- `.chart-series-7`: Discrete series color 7.
- `.chart-series-8`: Discrete series color 8.
- `.chart-highlight`: A selected or emphasized data mark.
- `.chart-muted`: A contextual or de-emphasized data mark.
- `.chart-positive`: A numerically positive value; positivity alone does not imply a favorable outcome.
- `.chart-negative`: A numerically negative value; negativity alone does not imply an unfavorable outcome.
- `.chart-line`: An unfilled SVG data line using the inherited series color and theme line width.
- `.chart-axis`: An unfilled SVG axis using the current secondary color.
- `.chart-grid`: An unfilled SVG reference line using the current border color.
- `.chart-label`: SVG text for categories, ticks, and other chart labels.
- `.chart-value`: SVG text for emphasized numeric values.
- `.chart-legend`: Apply to `ul` for a wrapping legend; direct `li` children align their keys and names.
- `.chart-legend-key`: A small HTML color key; combine with the corresponding chart color class on the same element.
- `.diagram-node`: An SVG node shape with a themed surface fill and primary stroke.
- `.diagram-edge`: An unfilled SVG connector; arrowheads require an explicit SVG marker.
- `.diagram-label`: SVG text intended to sit inside `.diagram-node` shapes, using their matching foreground.

> Chart color classes set both SVG fill and the inherited `--chart-series` variable. Use one color role per data mark, group, or legend key. Combine a color class with `.chart-line` on a path, or put the color on its parent group. Series numbers identify categories rather than rank; keep the same category-to-series mapping across related charts and legends.

### Public Variables

**Runtime supplies global variables through shared styles. Reference them in custom CSS without copying or redefining their values. Context variables may be assigned locally as described below.**

#### Slide canvas, surfaces, and foregrounds

- `--color-bg`: Slide background.
- `--color-fg`: Main text on the slide background.
- `--color-secondary`: Secondary text on the slide background.
- `--color-caption`: Supporting text on the slide background.
- `--color-primary`: Primary text emphasis and accents such as rules or outlines.
- `--color-accent`: Accent surface background.
- `--color-on-accent`: Main foreground on the accent surface.
- `--color-on-accent-secondary`: Secondary foreground on the accent surface.
- `--color-on-accent-caption`: Supporting foreground on the accent surface.
- `--color-surface`: Standard container background.
- `--color-on-surface`: Main foreground on the standard surface.
- `--color-on-surface-secondary`: Secondary foreground on the standard surface.
- `--color-on-surface-caption`: Supporting foreground on the standard surface.
- `--color-surface-soft`: Soft container background.
- `--color-on-surface-soft`: Main foreground on the soft surface.
- `--color-on-surface-soft-secondary`: Secondary foreground on the soft surface.
- `--color-on-surface-soft-caption`: Supporting foreground on the soft surface.
- `--color-border`: Default border and separator color.
- `--color-highlight`: Highlight background.
- `--color-on-highlight`: Foreground paired with the highlight background.

#### Semantic states

- `--color-success`: Success-state foreground.
- `--color-success-surface`: Background paired with the success foreground.
- `--color-success-border`: Success-state border.
- `--color-warning`: Warning-state foreground.
- `--color-warning-surface`: Background paired with the warning foreground.
- `--color-warning-border`: Warning-state border.
- `--color-danger`: Danger-state foreground.
- `--color-danger-surface`: Background paired with the danger foreground.
- `--color-danger-border`: Danger-state border.
- `--color-info`: Information-state foreground.
- `--color-info-surface`: Background paired with the information foreground.
- `--color-info-border`: Information-state border.

#### Typography

- `--font-sans`: Body font family stack.
- `--font-display`: Display and heading font family stack.
- `--font-mono`: Technical and monospace font family stack.
- `--text-title`: Main heading size.
- `--text-section`: Internal heading size.
- `--text-subtitle`: Lead text size.
- `--text-body`: Body text size.
- `--text-caption`: Supporting text size.
- `--text-metric`: Prominent metric size.
- `--weight-body`: Body text weight.
- `--weight-title`: Heading weight.
- `--weight-emphasis`: Emphasized text weight.
- `--leading-title`: Heading line height.
- `--leading-body`: Body line height.
- `--leading-caption`: Supporting text line height.
- `--tracking-title`: Heading letter spacing.

#### Spacing

- `--space-1`: Smallest general spacing step.
- `--space-2`: Small general spacing step.
- `--space-3`: Compact general spacing step.
- `--space-4`: Standard general spacing step.
- `--space-6`: Large general spacing step.
- `--space-8`: Largest general spacing step.
- `--stage-padding-x`: Horizontal safe inset for slide content.
- `--stage-padding-y`: Vertical safe inset for slide content.
- `--card-padding`: Inner spacing for cards and callouts.
- `--table-padding-x`: Horizontal table cell padding.
- `--table-padding-y`: Vertical table cell padding.
- `--list-gap`: Spacing between list items.
- `--metric-gap`: Spacing within a metric group.
- `--quote-padding`: Spacing between a quotation's rule and text.

#### Borders, radii, and shadows

- `--border-width`: Standard border width.
- `--border-style`: Standard CSS border style.
- `--quote-border-width`: Quotation rule width.
- `--radius-sm`: Small corner radius.
- `--radius-md`: Standard container radius.
- `--radius-lg`: Large corner radius.
- `--shadow-sm`: Subtle shadow.
- `--shadow-card`: Standard card shadow.
- `--shadow-lg`: Stronger shadow for elevated elements.

#### Chart colors and strokes

- `--chart-primary`: Primary data color.
- `--chart-secondary`: Secondary data color.
- `--chart-accent`: Accent data color.
- `--chart-series-1`: Discrete series color 1.
- `--chart-series-2`: Discrete series color 2.
- `--chart-series-3`: Discrete series color 3.
- `--chart-series-4`: Discrete series color 4.
- `--chart-series-5`: Discrete series color 5.
- `--chart-series-6`: Discrete series color 6.
- `--chart-series-7`: Discrete series color 7.
- `--chart-series-8`: Discrete series color 8.
- `--chart-highlight`: Selected or emphasized data color.
- `--chart-muted`: Contextual or de-emphasized data color.
- `--chart-positive`: Color for numerically positive values.
- `--chart-negative`: Color for numerically negative values.
- `--chart-line-width`: Data-line stroke width, expressed as a unitless SVG coordinate value.
- `--chart-grid-width`: Grid and axis stroke width, expressed as a unitless SVG coordinate value.

#### Context Variables

- `--content-bg`: Current container's background.
- `--content-fg`: Main foreground appropriate to the current container.
- `--content-secondary`: Secondary foreground appropriate to the current container.
- `--content-caption`: Supporting foreground appropriate to the current container.
- `--content-emphasis`: Emphasis color appropriate to the current container.
- `--content-border`: Border color appropriate to the current container.
- `--chart-series`: Current chart color, consumed by data lines and legend keys.

> These variables describe the current element or container. Public surfaces establish content colors for their descendants; chart color classes establish the current series color. Custom containers may set these variables locally using public variable references, keeping background and foreground colors paired. Setting variables alone does not draw a surface; apply them to the relevant CSS properties. Component-specific classes and variables are defined by each component's source; they are not additional public contracts.
