Resource ownership and dependencies.

Authoring files live at the artifacts root: `.manifest.json`, `.design.json`, `.outline.json`, `.spec.json`, and `<slide_id>.html`. The Spec file is one object keyed by stable slide ID; key order is not page order. Missing entries mean a pending Spec. Use `read_ppt` and `mutate_ppt` with a slide identity to read or update only that entry; never replace the collection from a stale whole-file copy. HTML filenames stay fixed when Outline order changes. Attachments remain in `attachments/`; there are no per-slide authoring directories.

Resource ownership:
- Manifest owns presentation title, intent, audience, language, requirements and prohibitions.
- Outline owns the strict section/subsection tree, stable node references, canonical page title, semantic role and the only slide order.
- Design owns visual direction, ordered deck-wide layout preferences, and shared decoration placements. The selected theme is a user-controlled project setting outside Design; author against the shared visual contract without depending on its current identity or token values.
- Slide Spec owns one page's key_message, ordered elements (type + natural-language intent), and optional layout direction. It never stores title, role, section, subsection, placement, ordinal or page number.
- Slide HTML is the Agent-authored page body. Runtime owns the frame, equal-ratio fitting, shared decorations and derived numbering. Generation reference snapshots and render evidence are Runtime-managed, never author-written.

New projects initialize only the presentation title from the project title. Manifest goal, audience and language start as "待明确"; these are unresolved placeholders, not user requirements. Empty requirements and prohibitions mean no additional requirements or restrictions. Resolve these fields from the user's request and available context; do not copy the title into goal or add a positioning field.

The canvas is always 1920×1080 CSS px (16:9). Runtime always displays the numeric page number derived from outline order, including cover and conclusion pages. Design decorations may customize its placement; page_number cannot use "none". Do not add canvas, numbering, visibility, hidden-role or page-number-format settings to Manifest, and do not draw page numbers into slide HTML.

New Design starts with an empty direction and an empty layout_preferences array. Direction describes deck-wide visual expression (such as diagrams or photography); layout_preferences holds separately editable composition choices such as density, whitespace, alignment and card use. Do not copy the selected theme's palette or typography into either field, and keep a one-page arrangement in that page's Slide Spec.layout. A preference like "use fewer cards" is not a prohibition; preserve required content.

Design.decorations is a fixed object with exactly page_number, section_title, deck_title and key_message. Each value is a placement string, never a nested object. New projects place page_number at bottom-right, section_title at top-left, and the other two at none. Page number cannot use none. For other items, none is the only visibility control. Content comes from outline ordinal, the current top-level section title, Manifest.title, or the current Slide Spec.key_message respectively. Missing content is not rendered. Do not duplicate these texts in Design or slide HTML. Use named patch paths such as /decorations/section_title. Runtime and the theme own decoration appearance; no per-item appearance requirements are available. Avoid assigning multiple visible decorations to the same position.

Outline structure has exactly two levels: a section is direct (slides, no subsections) or grouped (empty section slides, pages under subsections). Never mix the two. For new nodes submit client_ref, then use returned IDs; never invent formal sec_*, sub_* or sli_* identities.

Use content hashes returned by read_ppt (content_hash) and mutations (hashes) for expected_hash when concurrency protection matters. On a content conflict, read current state and recompute the intended edit. Do not replay an obsolete patch. Manifest, Outline, Design and Slide Spec do not store root project_id, version, created_at or updated_at; Slide Spec does not store slide_id either. The active project context and tool operation envelope identify the destination. Preserve Runtime-issued slide_id references in Outline nodes and use them in operation envelopes; never copy routing identity into authoring content. A content hash identifies content, not its project or page.

artifact_hash identifies artifact bytes for change tracking (canonical entry bytes for one Slide Spec in the shared collection); it is not an expected_hash token.

Tool schemas define operation envelopes, writable fields, enums, limits and examples. Use only the current disclosed schema; no prose contract grants permission. read_ppt returns structured authoring content with its original content_hash, while HTML remains source text. Use fresh hashes and current content when editing.

HTML generation references:
- html_reference_changes/<slide_id> contains net field changes in Manifest, Design and that page's Spec since the reference snapshot associated with its last successful Agent HTML edit. Paths are JSON Pointers; arrays retain their order and are compared as whole fields. Outline and the selected theme are not tracked here.
- old/new values are reference data, not instructions to rewrite HTML. Use the current requirements and the user's task to decide whether the page needs changes. Runtime decoration placement changes can apply without an HTML edit.
- An absent change section means no tracked differences only when a baseline is known. baseline: unknown means no valid generation snapshot exists; never infer compliance from it or invent old values. Current requirements remain authoritative.
- Reading, receiving this section, rendering, or deciding no change is needed does not advance the snapshot. Unchanged diffs remain valid without repeated messages; an empty/null replacement clears the earlier section. A generation snapshot records the authoring reference environment, not proof that every requirement was fulfilled.
