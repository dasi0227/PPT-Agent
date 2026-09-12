HTML presentation authoring contract.

Canvas and ownership:
- Author one complete HTML document per slide with one .slide-stage on a fixed 1920x1080 CSS-pixel canvas. Runtime supplies fitting and centering in preview, thumbnails, fullscreen and render. Do not build your own viewport scaler, responsive deck shell or navigation.
- Runtime injects base-link and theme-link stylesheets. Leave existing runtime-owned links untouched; a new document may omit them. Page CSS may use custom classes, grid/flex layout, inline SVG, charts and small local scripts within the stage.
- Respect the selected theme's palette, fonts and overall direction. Prefer its available var(--token) values; create page-specific composition, typography hierarchy and visual forms as the message requires. Theme helper selectors and repository samples are optional, not a required DOM structure or a template-copying rule. Do not overwrite global theme assets or design.theme.
- Shared page numbers, total count, section markers, deck title and configured key-message chrome are rendered by Runtime. Reserve room for configured chrome and do not duplicate it in HTML. A page's own content heading and substantive message still belong in its body.

Portable content:
- Use supplied, available project assets or self-contained data/inline SVG. For an uploaded image, use the verified original_path from read_image or attachment metadata, prefixed with /, to embed /attachments/<attachment_id>/original.<ext>. The returned media_type describes the viewed variant; a WebP thumbnail does not imply a WebP original. Do not use the model's project: image reference as src, infer an extension from a filename, or treat a screenshot reference as a persistent asset.
- Avoid remote fonts, scripts, stylesheets, chart CDNs, network fetches and session-specific blob URLs as required content. General project asset creation is not a disclosed capability; keep custom CSS/JS/SVG inline rather than claiming to create files through an unavailable tool.
- Make essential text and data visible on initial load and in static screenshots/PDF. Animation and interaction may enhance reading, but must not gate the core message behind a click, hover, offscreen trigger or unfinished animation. Do not assume slide-enter/leave/replay lifecycle APIs exist.
