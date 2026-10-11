---
id: playbook.html
description: Guides slide HTML creation and revision using content plans, visual requirements, public classes, theme variables, and reusable components.
---

## Procedure

1. **Understand the Change**: Use the request and relevant references to identify what needs to be created, revised, or recomposed. Read missing requirements, current HTML, and visual evidence needed for the implementation.
2. **Build the Implementation**: Translate the content and composition into semantic HTML using public classes, public CSS variables, suitable components, and page-scoped layout CSS.
3. **Save the HTML**: Use `edit_html` to save the implementation. Choose local edits or complete replacement according to the change and the tool contract; use current saved source for exact anchors.
4. **Inspect and Refine**: Call `render_slide` and inspect the returned image and diagnostics against the references and quality requirements. Correct the cause, save, and render the affected slide again.

## References

- **Spec**: Use the slide's message, content elements, and relationships to determine what the page presents and how its information is organized.
- **Design**: Use presentation-wide visual requirements to guide layout and styling, and reserve space for the configured shared decorations.
- **Existing HTML**: Use current source to understand the existing structure, styles, and behavior and identify the changes needed.
- **Screenshots**: Use current rendered screenshots to assess the visual result, locate problems, and compare revisions against spec and design.
- **Components**: Use relevant component source as a reference for structure, scoped styles, exposed variables, and behavior, adapting it to the slide's content and composition.

## Rules「IMPORTANT」

- **Runtime**: Runtime isolates each slide, handles canvas scaling and centering, and loads fonts, public class definitions, and public CSS variables before page-specific styles. It renders shared decorations from project content at design's configured positions and derives page numbers from slide order. On theme changes, it updates public styles and variables and dispatches `ppt:themechange`; scripts that cache computed styles must refresh those values and redraw on this event. Reserve space for shared decorations and do not duplicate them in authored HTML.
- **HTML Document**: Write a separate, complete HTML document for each slide, with `<!doctype html>`, a language attribute, UTF-8 metadata, and exactly one `.slide-stage` as its fixed 1920 × 1080 CSS-pixel canvas. Keep the slide's custom CSS, SVG, and scripts within that document. Do not combine multiple slides into one document or add viewport scaling, deck navigation, or a responsive canvas shell.
- **Public Classes**: Use public classes for the roles and combinations defined in Contracts. Combine them with page-specific classes for layout and necessary adjustments to size or spacing. Scope custom selectors to the page or component, and reuse existing public styling rather than recreating it. Do not redefine public classes globally.
- **Public Variables**: Use public CSS variables for custom styling. Inside a container, use inherited `--content-*` variables for text, emphasis, and borders. A custom surface must establish matching content variables for its children. Keep variable references through `var()`, `calc()`, or `color-mix()` instead of copying resolved values, and do not reset public variables globally. Use fixed colors only when required by the content, such as brand marks or factual color encodings.
- **Component Adaptation**: Adapt a component's content, size, and placement through its exposed variables and scoped selectors while preserving its defining structure and behavior. Use component-specific class names and scope styles to the component. Retain the fallback chain through component-specific, inherited content, and public CSS variables to standalone defaults. Do not replace these references with resolved values or add public classes indiscriminately to component internals.
- **Portable Assets**: Use available project assets through their supplied references, following the image reference rules. Use inline SVG or data for authored graphics. Do not make essential content depend on remote fonts, stylesheets, scripts, chart CDNs, network fetches, or temporary blob URLs.
- **Static Output**: Show essential content on initial load and in screenshots or PDF. Interaction and animation must leave a complete static and reduced-motion presentation; do not gate content behind clicks, hover, scrolling, or an unfinished animation. Do not assume slide-enter, leave, or replay APIs exist.

## Contracts「IMPORTANT」

### Public Classes

> Public classes supply appearance and a few content structures; page CSS supplies the composition. Modifiers apply only where described. Their names do not imply that other classes are automatically added to the element or its children.

**Canvas and text**

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

**Surfaces and states**

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

Use at most one card variant on a card. Apply one tone on the same element as `.status`, `.callout`, `.metric-change`, or a plain `.card`; tones set variables and do not draw a standalone container. Avoid combining a tone with a card variant.

**Content and images**

- `.metric`: A vertical group for a value, label, and optional change indicator.
- `.metric-value`: A prominent numeric value with tabular figures; usable inside or outside `.metric`.
- `.metric-label`: The supporting label for a metric.
- `.metric-unit`: A smaller secondary unit beside or within a metric value.
- `.metric-change`: A compact change indicator; combine with a tone when the meaning warrants it.
- `.quote`: A quotation with display typography and an emphasis rule.
- `.slide-list`: Apply to `ul` or `ol` for theme-sized text, markers, and item spacing; nested lists can use it too.
- `.image-frame`: A clipped image container with a themed background, border, and radius, typically a `figure` or `div`.
- `.image-cover`: Apply directly to an image to fill its sized container while preserving aspect ratio and cropping excess content.
- `.image-contain`: Apply directly to an image to fit the whole image inside its sized container while preserving aspect ratio.
- `.pill`: A compact category or tag label with an outline.
- `.divider`: A thematic separator, typically an `hr`.

Choose either `.image-cover` or `.image-contain` for an image. A direct image child of `.image-frame` otherwise uses full width and its natural aspect ratio.

**Tables**

- `.data-table`: Apply to `table` for full-width tabular content, cell padding, aligned headers, and row separators.
- `.table-numeric`: Apply to numeric `th` and `td` descendants of `.data-table` for end alignment and tabular figures; include the column header and each numeric cell.
- `.table-focus`: Apply to a `tr`, `th`, or `td` inside `.data-table` to highlight that row or cell with paired colors.
- `.table-striped`: Combine with `.data-table` on the same table for alternating body-row backgrounds.
- `.table-bordered`: Combine with `.data-table` on the same table for borders around every cell; may also be combined with `.table-striped`.
- `.data-table > tfoot`: A standard table footer receives summary styling and a stronger top separator automatically; no additional class is required.

**Charts and diagrams**

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

Chart color classes set both SVG fill and the inherited `--chart-series` variable. Use one color role per data mark, group, or legend key. Combine a color class with `.chart-line` on a path, or put the color on its parent group. Series numbers identify categories rather than rank; keep the same category-to-series mapping across related charts and legends.

### Theme Variables

Runtime supplies the active theme's values. Reference these variables directly in custom CSS; do not copy their definitions into slide HTML.

**Page, surfaces, and foregrounds**

- `--color-bg`: Page background.
- `--color-fg`: Main text on the page background.
- `--color-secondary`: Secondary text on the page background.
- `--color-caption`: Supporting text on the page background.
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

**Semantic states**

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

**Typography**

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

**Spacing**

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

**Borders, radii, and shadows**

- `--border-width`: Standard border width.
- `--border-style`: Standard CSS border style.
- `--quote-border-width`: Quotation rule width.
- `--radius-sm`: Small corner radius.
- `--radius-md`: Standard container radius.
- `--radius-lg`: Large corner radius.
- `--shadow-sm`: Subtle shadow.
- `--shadow-card`: Standard card shadow.
- `--shadow-lg`: Stronger shadow for elevated elements.

**Chart colors and strokes**

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

### Inherited Local Variables

These variables describe the current element or container, not a separate global theme. Public surfaces establish content colors for their descendants; chart color classes establish the current series color.

- `--content-bg`: Current container's background.
- `--content-fg`: Main foreground appropriate to the current container.
- `--content-secondary`: Secondary foreground appropriate to the current container.
- `--content-caption`: Supporting foreground appropriate to the current container.
- `--content-emphasis`: Emphasis color appropriate to the current container.
- `--content-border`: Border color appropriate to the current container.
- `--chart-series`: Current chart color, consumed by data lines and legend keys.

Component-specific classes and variables are defined by each component's source; they are not additional public contracts.

## Examples

These examples demonstrate implementation patterns, not page templates. The first is a document skeleton; the others are fragments to incorporate into a slide's head and stage as appropriate. Replace illustrative text, data, labels, and asset references with the actual project material.

### Document and Local Layout

Combine public text roles with a page-specific layout. Keep content order meaningful without relying on absolute positioning.

```html
<!doctype html>
<html lang="zh-CN">
<head>
  <meta charset="utf-8">
  <title>验证结果</title>
  <style>
    .page-results .results-layout {
      display: grid;
      align-content: start;
      gap: var(--space-8);
    }
    .page-results .results-evidence {
      display: grid;
      grid-template-columns: 1.2fr 1fr;
      gap: calc(var(--space-8) * 2);
      align-items: start;
    }
    .page-results .results-quote { margin: 0; display: grid; gap: var(--space-3); }
  </style>
</head>
<body>
  <main class="slide-stage page-results">
    <section class="slide-content results-layout">
      <h1 class="slide-title">验证结果</h1>
      <div class="results-evidence">
        <ul class="slide-list">
          <li>主要发现及其依据</li>
          <li>仍需验证的条件</li>
        </ul>
        <figure class="results-quote">
          <blockquote class="quote">来自实际材料的关键引述</blockquote>
          <figcaption class="caption">引述来源</figcaption>
        </figure>
      </div>
    </section>
  </main>
</body>
</html>
```

### Surface Inheritance, Metrics, and States

Public metric text follows the accent surface automatically. Custom text uses local content variables. Apply a tone to the status element itself to establish a separate, paired status surface.

```html
<style>
  .page-summary .summary-panel { display: grid; gap: var(--space-6); }
  .page-summary .summary-detail {
    color: var(--content-secondary);
    border-top: var(--border-width) var(--border-style) var(--content-border);
    padding-top: var(--space-4);
  }
</style>
<section class="page-summary">
  <div class="card card-accent summary-panel">
    <h2 class="section-title">验证进度</h2>
    <div class="metric">
      <p class="metric-value">68<span class="metric-unit">%</span></p>
      <p class="metric-label">已验证事项占比</p>
      <p class="metric-change">较上期 +12 个百分点</p>
    </div>
    <p class="summary-detail">剩余事项依赖下一轮样本。</p>
    <p><span class="status tone-warning">待补充样本</span></p>
  </div>
</section>
```

### Numeric Tables and Summaries

Table-wide modifiers go on the table. Numeric alignment goes on the corresponding header and cells. Use standard `tfoot` markup for totals and a descendant modifier for a highlighted row or cell.

```html
<table class="data-table table-striped table-bordered">
  <thead>
    <tr>
      <th scope="col">阶段</th>
      <th scope="col" class="table-numeric">完成量</th>
      <th scope="col" class="table-numeric">占比</th>
    </tr>
  </thead>
  <tbody>
    <tr><th scope="row">探索</th><td class="table-numeric">40</td><td class="table-numeric">40%</td></tr>
    <tr><th scope="row">验证</th><td class="table-numeric table-focus">60</td><td class="table-numeric">60%</td></tr>
  </tbody>
  <tfoot>
    <tr><th scope="row">合计</th><td class="table-numeric">100</td><td class="table-numeric">100%</td></tr>
  </tfoot>
</table>
```

### SVG Data Marks, Lines, and Legends

Keep data coordinates independent of theme colors. The bars and their legend keys share a series class; the reference line inherits its color from an SVG group. Numeric labels remain separate from category labels.

```html
<svg viewBox="0 0 760 380" width="760" height="380" role="img"
     aria-label="示例：方案 A 完成 40，方案 B 完成 60，参考值为 50。">
  <path class="chart-axis" d="M80 30V300H720" />
  <path class="chart-grid" d="M80 220H720 M80 140H720 M80 60H720" />
  <g class="chart-label" text-anchor="end">
    <text x="64" y="308">0</text>
    <text x="64" y="68">60</text>
  </g>
  <rect class="chart-series-1" x="160" y="140" width="120" height="160" />
  <rect class="chart-series-2" x="420" y="60" width="120" height="240" />
  <g class="chart-series-3">
    <path class="chart-line" d="M80 100H720" stroke-dasharray="8 6" />
  </g>
  <g class="chart-value" text-anchor="middle">
    <text x="220" y="128">40</text>
    <text x="480" y="48">60</text>
  </g>
  <g class="chart-label" text-anchor="middle">
    <text x="220" y="346">方案 A</text>
    <text x="480" y="346">方案 B</text>
  </g>
</svg>
<ul class="chart-legend">
  <li><span class="chart-legend-key chart-series-1" aria-hidden="true"></span>方案 A</li>
  <li><span class="chart-legend-key chart-series-2" aria-hidden="true"></span>方案 B</li>
  <li><span class="chart-legend-key chart-series-3" aria-hidden="true"></span>参考值 50</li>
</ul>
```

### Diagram Nodes and Connectors

Define arrowheads explicitly, keep their color bound to the connector's context, and place node labels on the node surface. Use unique IDs when incorporating multiple diagrams.

```html
<svg viewBox="0 0 760 160" width="760" height="160" role="img"
     aria-label="收集证据，然后验证结论。">
  <defs>
    <marker id="evidence-arrow" viewBox="0 0 10 10" refX="10" refY="5"
            markerWidth="8" markerHeight="8" orient="auto-start-reverse">
      <path d="M0 0L10 5L0 10Z" fill="var(--content-secondary)" />
    </marker>
  </defs>
  <path class="diagram-edge" d="M280 80H470" marker-end="url(#evidence-arrow)" />
  <rect class="diagram-node" x="20" y="30" width="260" height="100" />
  <rect class="diagram-node" x="470" y="30" width="260" height="100" />
  <g class="diagram-label" text-anchor="middle" dominant-baseline="middle">
    <text x="150" y="80">收集证据</text>
    <text x="600" y="80">验证结论</text>
  </g>
</svg>
```

### Image Frames and Cropping

Keep the caption outside the clipped image container. Replace the asset placeholder with a supplied, verified reference. Use `.image-contain` instead when the full image must remain visible.

```html
<style>
  .page-fieldwork .fieldwork-figure { margin: 0; display: grid; gap: var(--space-3); }
  .page-fieldwork .fieldwork-photo { width: 720px; height: 420px; }
  .page-fieldwork .fieldwork-photo img { object-position: center 40%; }
</style>
<section class="page-fieldwork">
  <figure class="fieldwork-figure">
    <div class="image-frame fieldwork-photo">
      <img class="image-cover" src="VERIFIED_ASSET_REFERENCE" alt="现场测试中的设备与操作位置">
    </div>
    <figcaption class="caption">现场照片及其来源</figcaption>
  </figure>
</section>
```

### Component Adaptation

After incorporating the actual `stat-badge` source and styles, adapt this instance through its exposed size variable. Its existing foreground and emphasis bindings inherit from the accent card; no additional public classes are needed on the component's internal elements.

```html
<style>
  .page-impact .ppt-stat-badge {
    --stat-badge-size: calc(var(--text-metric) * 0.85);
    max-width: none;
  }
</style>
<section class="page-impact">
  <div class="card card-accent">
    <div class="ppt-stat-badge">
      <strong class="ppt-stat-badge__value">42%</strong>
      <span class="ppt-stat-badge__label">流程效率提升</span>
    </div>
  </div>
</section>
```

### Canvas Redraw on Theme Changes

Resolve colors from the containing surface, and resolve them again whenever the theme changes. Keep a textual representation of the data available alongside the canvas.

```html
<div id="completion-chart" class="card">
  <p class="slide-body">已完成 60%，剩余 40%。</p>
  <canvas width="600" height="80" aria-hidden="true"></canvas>
</div>
<script>
  (() => {
    const host = document.getElementById('completion-chart');
    const canvas = host.querySelector('canvas');
    const context = canvas.getContext('2d');
    const probe = document.createElement('span');
    probe.hidden = true;
    host.append(probe);

    function redraw() {
      if (!context) return;
      const color = variable => {
        probe.style.color = `var(${variable})`;
        return getComputedStyle(probe).color;
      };
      context.clearRect(0, 0, canvas.width, canvas.height);
      context.fillStyle = color('--chart-muted');
      context.fillRect(0, 20, 600, 40);
      context.fillStyle = color('--chart-primary');
      context.fillRect(0, 20, 360, 40);
    }

    window.addEventListener('ppt:themechange', redraw);
    redraw();
  })();
</script>
```
