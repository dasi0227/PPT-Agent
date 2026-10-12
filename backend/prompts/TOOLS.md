## read_resource

### 工具描述

Read one existing resource. Spec and html require slide_id; global resources forbid it. project_state reports missing resources, so do not read them just to confirm absence; use edit_manifest or edit_design to create missing requirements, and edit_outline(init) to create a missing outline. Read OUTLINE for existing titles, hierarchy and page numbering.

### 输入参数

#### resource

Resource to read: manifest for presentation goals and content requirements, design for global visual requirements, outline for page titles and order, spec for one page's semantic design, or html for its source.

#### slide_id

Stable page ID from the id field of a page in the current Outline. Required for spec or html; omit for manifest, design and outline.

### 输出参数

The complete current resource, without version hashes or a data wrapper.

#### oneOf[0]

Requested global resource has not been created. Use its edit tool to initialize it; do not reread it without a write.

#### oneOf[0].content

No saved content exists.

#### oneOf[0].resource

Missing global resource.

#### oneOf[0].status

The source file does not exist.

#### oneOf[1]

Result for manifest.

#### oneOf[1].content

Complete saved manifest JSON object.

#### oneOf[1].content.audience

Who the presentation is for and their relevant background or knowledge.

#### oneOf[1].content.goal

The intended audience outcome: what they should understand, decide or be able to do. Keep detailed content coverage in requirements.

#### oneOf[1].content.language

The presentation language, such as zh-CN. Do not repeat it in requirements.

#### oneOf[1].content.pages

Desired slide count, such as 10, 11-12 or 约10; 待明确 when unresolved. This is a planning requirement, not the actual count derived from Outline. Do not repeat it in requirements.

#### oneOf[1].content.prohibitions

Explicit content exclusions and prohibitions; empty when none are specified.

#### oneOf[1].content.prohibitions[]

One concrete content requirement or exclusion, according to the containing field; do not put visual settings here.

#### oneOf[1].content.requirements

Additional required content, evidence and wording conventions; empty when none are specified.

#### oneOf[1].content.requirements[]

One concrete content requirement or exclusion, according to the containing field; do not put visual settings here.

#### oneOf[1].content.title

The presentation title, not a summary of its goals or requirements.

#### oneOf[1].resource

Resource type actually read; determines the content format.

#### oneOf[2]

Result for design.

#### oneOf[2].content

Complete saved design JSON object.

#### oneOf[2].content.decorations

Saved positions of shared decorations. Every non-none position is unique across all configured decorations, even if text is currently missing. Use none to hide an optional decoration. Text derives from presentation resources; appearance comes from Runtime and the theme.

#### oneOf[2].content.decorations.deck_title

Placement of the presentation title from Manifest.title. Use none to hide it; missing text is not rendered.

#### oneOf[2].content.decorations.key_message

Placement of the current page's core message from its Spec.core. Use none to hide it; missing text is not rendered.

#### oneOf[2].content.decorations.page_number

Placement of the numeric page number derived from outline order. It is always shown, including on cover and conclusion pages; none is not allowed.

#### oneOf[2].content.decorations.section_title

Placement of the current top-level section title from the outline, not the subsection title. Use none to hide it; missing text is not rendered.

#### oneOf[2].content.demands

Visual design requirements that page authoring must follow throughout this presentation. Record independently actionable natural-language requirements covering overall look, information density, layout and hierarchy, text/image expression, use of visual elements, reference and brand constraints, or motion. Items may include conditions, preferences or explicit prohibitions; do not require every dimension. Record actual project requirements only, not generic quality rules or copies of the selected theme's existing configuration. Keep content requirements in Manifest and a single page's arrangement in its Spec. Explicit palette or typography requirements must be fulfilled through available theme capabilities; explain unsupported requests. Changing this list updates authoring guidance, not existing HTML. Supplying it replaces the list completely; use [] when unspecified or to clear it.

#### oneOf[2].content.demands[]

One independent, explicit and actionable deck-wide visual design requirement, optionally with conditions, a preference or a prohibition.

#### oneOf[2].resource

Resource type actually read; determines the content format.

#### oneOf[3]

Result for spec.

#### oneOf[3].content

Complete saved spec JSON object.

#### oneOf[3].content.content_type

Optional dominant communication mode of a body page: explanation explains concepts, definitions, principles or facts; comparison compares objects, differences and trade-offs; example illustrates through concrete cases or examples; guidance provides methods, steps or action recommendations; other means a known body-content mode outside these categories, such as an exercise or task. Allowed only when the saved purpose is content. Omit for mixed content or an undetermined mode; never default to other, and do not use other for unknown or insufficient information.

#### oneOf[3].content.core

The main takeaway the audience should retain from this page, not its title or a list of layout instructions.

#### oneOf[3].content.elements

Ordered content elements planned for this page; an empty array means no elements are specified. Each item pairs representation and communication intent, not HTML.

#### oneOf[3].content.elements[]

One planned content element, pairing a representation type with its communication intent.

#### oneOf[3].content.elements[].intent

What this element should communicate: the relevant content, evidence or relationship and its purpose on the page. Describe the intended content in natural language without inventing missing facts.

#### oneOf[3].content.elements[].type

Representation of this element: text for prose, list for items, metric for a key figure, quote for a quotation, table for tabular values, chart for data, diagram for relationships, code for a code sample, or asset for visual media. comparison is a page content_type; choose chart, table or diagram for a comparison element.

#### oneOf[3].content.layout

Natural-language suggestion for arranging this page's content and reading order, such as a chart on the left and explanation on the right; not a template identifier or CSS.

#### oneOf[3].content.purpose

Optional purpose of this page in the narrative: cover presents the topic and identity; introduction introduces the topic, background, problem, goals or structure; transition switches chapters or topics; content develops the body; conclusion closes with a summary, conclusions or action recommendations; other means a known purpose outside these categories. This is not a visual template. Omit when undetermined; never default to content or other. A saved content_type is allowed only when purpose is content.

#### oneOf[3].resource

Resource type actually read; determines the content format.

#### oneOf[3].slide_id

Stable ID of the page read.

#### oneOf[4]

Result for outline.

#### oneOf[4].content

Exact saved outline JSON source string, not a parsed object; use directly for exact edits. sections contains ordered top-level groups with id, title, purpose, slides and subsections. Subsections have id, title, purpose and slides; each page has id and title. title is the visible heading of a section, subsection or page; purpose explains a section or subsection's contribution to the narrative. IDs are stable backend-assigned identities, not positions; page IDs have the sli_ prefix and are passed as slide_id to page tools. A section uses direct slides or subsections, not both; unused arrays are empty.

#### oneOf[4].resource

Resource type actually read; determines the content format.

#### oneOf[5]

Result for html.

#### oneOf[5].content

Exact saved HTML source text, suitable for subsequent exact text edits.

#### oneOf[5].resource

Resource type actually read; determines the content format.

#### oneOf[5].slide_id

Stable ID of the page read.

## render_slide

### 工具描述

Render one authorized slide in isolated Chromium, reusing its existing screenshot and diagnostics when all render inputs are unchanged. New style inspections run when their selectors are not already covered. Only each page's latest valid screenshot is retained in model context; older screenshots remain in display history. Use read_image(slide_id) to revisit a valid screenshot.

### 输入参数

#### inspect_selectors

Optional CSS selectors for elements with a concrete unresolved visual defect. Returns current computed colors, background layers, typography and SVG fill/stroke with the screenshot, up to three matches per selector. Use to diagnose CSS cascade or contrast instead of guessing successive replacements; omit for ordinary renders.

#### inspect_selectors[]

One CSS selector from the current saved HTML identifying the problem element, e.g. .bad .vs-tag .kicker. Invalid selectors are reported in diagnostics without canceling the screenshot.

#### slide_id

Stable ID of a page with saved HTML to render and inspect, within the current run's authorized page scope. Pass the page's id from the current Outline as slide_id, not a page number.

### 输出参数

Successful screenshot image block plus this JSON diagnostic text. The image is not a JSON field or a path; inspect it because diagnostics alone do not prove visual correctness.

#### diagnostics

Observed rendering problems; these checks do not prove complete visual correctness.

#### diagnostics.console_errors

Up to 50 console error messages or uncaught script errors, at most 1000 characters each; empty if none.

#### diagnostics.console_errors[]

Captured browser error message.

#### diagnostics.failed_resources

Up to 50 deduplicated failed requests, HTTP 400+ responses or blocked external resources; empty if none.

#### diagnostics.failed_resources[]

Failure cause or HTTP status with the resource address.

#### diagnostics.out_of_bounds

Up to 50 descendants extending beyond .slide-stage (document root if absent), allowing 1px tolerance; empty if none. Does not detect all clipping, ellipsis or overlap.

#### diagnostics.out_of_bounds[]

One element whose bounds extend outside the stage.

#### diagnostics.out_of_bounds[].class

Element class text, when present, truncated to at most 160 characters.

#### diagnostics.out_of_bounds[].id

Element ID, omitted when absent.

#### diagnostics.out_of_bounds[].rect

Element bounding rectangle in the page iframe viewport.

#### diagnostics.out_of_bounds[].rect.bottom

Element's bottom edge relative to the page iframe viewport, in CSS pixels rounded to an integer.

#### diagnostics.out_of_bounds[].rect.left

Element's left edge relative to the page iframe viewport, in CSS pixels rounded to an integer.

#### diagnostics.out_of_bounds[].rect.right

Element's right edge relative to the page iframe viewport, in CSS pixels rounded to an integer.

#### diagnostics.out_of_bounds[].rect.top

Element's top edge relative to the page iframe viewport, in CSS pixels rounded to an integer.

#### diagnostics.out_of_bounds[].tag

Lowercase HTML tag name.

#### diagnostics.overflow

Scrollable content size compared to canvas viewport with 1px tolerance; independent of individual element bounds.

#### diagnostics.overflow.horizontal

Horizontal overflow verdict with content width, canvas width and excess pixels when overflowing.

#### diagnostics.overflow.vertical

Vertical overflow verdict with content height, canvas height and excess pixels when overflowing.

#### diagnostics.style_inspections

Present only when inspect_selectors was requested. Advisory computed styles identify CSS effects; they are not a contrast score, the originating CSS rule, or visual approval.

#### diagnostics.style_inspections[]

One requested selector's result.

#### diagnostics.style_inspections[].elements

Up to three matched elements in document order.

#### diagnostics.style_inspections[].elements[]

Actual style values of one matched element.

#### diagnostics.style_inspections[].elements[].background_layers

Up to eight nontransparent or image-bearing layers from this element through its ancestors, nearest first. These are CSS layers, not a composited pixel color; transparency, images and pseudo-elements may require screenshot inspection.

#### diagnostics.style_inspections[].elements[].background_layers[]

One element or ancestor background layer.

#### diagnostics.style_inspections[].elements[].background_layers[].class

Layer owner class text, capped at 160 characters.

#### diagnostics.style_inspections[].elements[].background_layers[].color

Computed background color.

#### diagnostics.style_inspections[].elements[].background_layers[].id

Layer owner ID.

#### diagnostics.style_inspections[].elements[].background_layers[].image

Computed background image.

#### diagnostics.style_inspections[].elements[].background_layers[].opacity

Computed layer owner's opacity.

#### diagnostics.style_inspections[].elements[].background_layers[].tag

Layer owner tag.

#### diagnostics.style_inspections[].elements[].class

Element class text, capped at 160 characters.

#### diagnostics.style_inspections[].elements[].computed

Computed style after theme and local CSS are applied.

#### diagnostics.style_inspections[].elements[].computed.background_color

Computed background color on this element; may be transparent.

#### diagnostics.style_inspections[].elements[].computed.background_image

Computed background image, including gradients; color alone may not describe the visible surface.

#### diagnostics.style_inspections[].elements[].computed.color

Computed foreground color.

#### diagnostics.style_inspections[].elements[].computed.display

Computed display value.

#### diagnostics.style_inspections[].elements[].computed.fill

Computed SVG fill.

#### diagnostics.style_inspections[].elements[].computed.font_size

Computed font size.

#### diagnostics.style_inspections[].elements[].computed.line_height

Computed line height.

#### diagnostics.style_inspections[].elements[].computed.opacity

Computed opacity.

#### diagnostics.style_inspections[].elements[].computed.stroke

Computed SVG stroke.

#### diagnostics.style_inspections[].elements[].computed.visibility

Computed visibility.

#### diagnostics.style_inspections[].elements[].id

Element ID, empty when absent.

#### diagnostics.style_inspections[].elements[].tag

Lowercase element tag.

#### diagnostics.style_inspections[].error

Selector parsing error, when invalid; the screenshot remains available.

#### diagnostics.style_inspections[].match_count

Total matches; only the first three are inspected. Zero means no matching element or an invalid selector.

#### diagnostics.style_inspections[].selector

CSS selector requested for this page.

#### slide_id

Stable ID of the rendered page.

## read_image

### 工具描述

Read an uploaded original image by attachment_id, or the latest valid screenshot by slide_id. Supply exactly one. Missing or stale screenshots require render_slide first. Uploaded originals and each page's latest valid screenshot are retained through context compaction and recovery within the current run; older screenshots remain in display history only. Use attachment context addresses for HTML embedding.

### 输入参数

#### attachment_id

ID of an uploaded image from attachment context, used to view its original image. Supply this or slide_id, never both; do not pass a path or URL.

#### slide_id

Stable page ID whose latest valid rendered screenshot should be viewed. Supply this or attachment_id, never both. If no current screenshot exists, call render_slide first.

### 输出参数

On success, only the actual image content block is returned: uploaded image or latest valid page screenshot. There is no JSON image field, path, URL or metadata text.

## load_component

### 工具描述

Load enabled repository component HTML references by stable ids for adaptation. Loads the whole batch or fails, with at most 192 KiB of HTML per call. Loaded HTML remains available in active_components on subsequent requests; this does not modify project files.

### 输入参数

#### ids

Distinct enabled component IDs selected from the supplied component catalog, to load their HTML for reference and adaptation.

#### ids[]

A component's stable catalog ID, not its name or a file path.

### 输出参数

Requested repository bodies, partitioned by visibility in the current model context.

#### already_available

IDs whose full bodies are already in the current model context; catalog listings alone do not count.

#### already_available[]

Requested entry ID whose body is not repeated.

#### components

Entries whose full bodies are newly supplied; empty if all are already visible.

#### components[]

One requested repository entry.

#### components[].content

Complete component HTML source for reference and adaptation.

#### components[].id

Stable catalog ID of this entry.

## load_skill

### 工具描述

Load one or more enabled repository skills into the current run active skill set. Their full instructions remain available in active_skills on subsequent requests.

### 输入参数

#### ids

Distinct enabled skill IDs selected from the supplied skill catalog, to make their instructions available for the current run.

#### ids[]

A skill's stable catalog ID, not its name or a file path.

### 输出参数

Requested repository bodies, partitioned by visibility in the current model context.

#### already_available

IDs whose full bodies are already in the current model context; catalog listings alone do not count.

#### already_available[]

Requested entry ID whose body is not repeated.

#### skills

Entries whose full bodies are newly supplied; empty if all are already visible.

#### skills[]

One requested repository entry.

#### skills[].content

Complete skill instructions made available to this run.

#### skills[].id

Stable catalog ID of this entry.

## edit_manifest

### 工具描述

Edit supplied top-level fields; omitted fields remain unchanged, arrays replace completely. If the manifest file is absent, this tool creates it using the project title and default '待明确' required fields before applying supplied fields. Creation and edits require user approval before saving.

### 输入参数

#### audience

Who the presentation is for and their relevant background or knowledge.

#### goal

The intended audience outcome: what they should understand, decide or be able to do. Keep detailed content coverage in requirements.

#### language

The presentation language, such as zh-CN. Do not repeat it in requirements.

#### pages

Desired slide count, such as 10, 11-12 or 约10; 待明确 when unresolved. This is a planning requirement, not the actual count derived from Outline. Do not repeat it in requirements.

#### prohibitions

Explicit content exclusions and prohibitions. Do not restate the basics or positive requirements, and do not store visual or theme settings here. Supplying this list replaces it completely; [] clears it.

#### prohibitions[]

One concrete content requirement or exclusion, according to the containing field; do not put visual settings here.

#### requirements

Additional required content, examples, evidence and wording conventions. Do not repeat title, language, pages, audience or goal; exclude visual direction, layout and theme settings. Preserve any extra actionable detail when separating mixed requirements. Supplying this list replaces it completely; [] clears it.

#### requirements[]

One concrete content requirement or exclusion, according to the containing field; do not put visual settings here.

#### title

The presentation title, not a summary of its goals or requirements.

### 输出参数

Normal reply: the requested operation completed, or the user refused it. A refusal does not apply the operation or satisfy calls that require its effect.

#### oneOf[0]

Saved resource; omitted input fields are included with their retained values.

#### oneOf[0].changed

Whether this call actually changed saved content. False means no repair was applied; do not treat it as a fixed defect or repeat the same edit.

#### oneOf[0].changed_fields

Top-level fields whose stored values changed; empty for a no-op.

#### oneOf[0].changed_fields[]

Changed field name in content.

#### oneOf[0].content

Complete saved manifest JSON object.

#### oneOf[0].content.audience

Who the presentation is for and their relevant background or knowledge.

#### oneOf[0].content.goal

The intended audience outcome: what they should understand, decide or be able to do. Keep detailed content coverage in requirements.

#### oneOf[0].content.language

The presentation language, such as zh-CN. Do not repeat it in requirements.

#### oneOf[0].content.pages

Desired slide count, such as 10, 11-12 or 约10; 待明确 when unresolved. This is a planning requirement, not the actual count derived from Outline. Do not repeat it in requirements.

#### oneOf[0].content.prohibitions

Explicit content exclusions and prohibitions; empty when none are specified.

#### oneOf[0].content.prohibitions[]

One concrete content requirement or exclusion, according to the containing field; do not put visual settings here.

#### oneOf[0].content.requirements

Additional required content, evidence and wording conventions; empty when none are specified.

#### oneOf[0].content.requirements[]

One concrete content requirement or exclusion, according to the containing field; do not put visual settings here.

#### oneOf[0].content.title

The presentation title, not a summary of its goals or requirements.

#### oneOf[1]

The user refused this operation. It was not applied; no edit was committed or command executed.

#### oneOf[1].decision

The user declined this operation; this is a normal decision, not a tool error.

#### oneOf[1].feedback

Optional verbatim user reason or suggestions. Use it to revise the approach within authorization; do not repeat the denied intent unchanged or treat feedback as approval.

#### oneOf[1].summary

Acknowledgement that the operation was refused and did not take effect.

## edit_design

### 工具描述

Edit supplied top-level fields; omitted fields remain unchanged, arrays replace completely. Decorations merge only the supplied position fields. If the design file is absent, this tool creates it from empty demands and default decorations before applying supplied fields. Creation and edits require user approval before saving.

### 输入参数

#### decorations

Shared decoration positions to change. Only supplied keys are merged; omitted positions remain unchanged. Each non-none position may belong to only one configured decoration, even if its text is currently missing. The merged configuration is validated; conflicts are rejected. Move or hide the occupying decoration in the same edit when reassigning its position. Text comes from presentation resources; Runtime and the theme control appearance.

#### decorations.deck_title

Placement of the presentation title from Manifest.title. Use none to hide it; missing text is not rendered.

#### decorations.key_message

Placement of the current page's core message from its Spec.core. Use none to hide it; missing text is not rendered.

#### decorations.page_number

Placement of the numeric page number derived from outline order. It is always shown, including on cover and conclusion pages; none is not allowed.

#### decorations.section_title

Placement of the current top-level section title from the outline, not the subsection title. Use none to hide it; missing text is not rendered.

#### demands

Visual design requirements that page authoring must follow throughout this presentation. Record independently actionable natural-language requirements covering overall look, information density, layout and hierarchy, text/image expression, use of visual elements, reference and brand constraints, or motion. Items may include conditions, preferences or explicit prohibitions; do not require every dimension. Record actual project requirements only, not generic quality rules or copies of the selected theme's existing configuration. Keep content requirements in Manifest and a single page's arrangement in its Spec. Explicit palette or typography requirements must be fulfilled through available theme capabilities; explain unsupported requests. Changing this list updates authoring guidance, not existing HTML. Supplying it replaces the list completely; use [] when unspecified or to clear it.

#### demands[]

One independent, explicit and actionable deck-wide visual design requirement, optionally with conditions, a preference or a prohibition.

### 输出参数

Normal reply: the requested operation completed, or the user refused it. A refusal does not apply the operation or satisfy calls that require its effect.

#### oneOf[0]

Saved resource; omitted input fields are included with their retained values.

#### oneOf[0].changed

Whether this call actually changed saved content. False means no repair was applied; do not treat it as a fixed defect or repeat the same edit.

#### oneOf[0].changed_fields

Top-level fields whose stored values changed; empty for a no-op.

#### oneOf[0].changed_fields[]

Changed field name in content.

#### oneOf[0].content

Complete saved design JSON object.

#### oneOf[0].content.decorations

Saved positions of shared decorations. Every non-none position is unique across all configured decorations, even if text is currently missing. Use none to hide an optional decoration. Text derives from presentation resources; appearance comes from Runtime and the theme.

#### oneOf[0].content.decorations.deck_title

Placement of the presentation title from Manifest.title. Use none to hide it; missing text is not rendered.

#### oneOf[0].content.decorations.key_message

Placement of the current page's core message from its Spec.core. Use none to hide it; missing text is not rendered.

#### oneOf[0].content.decorations.page_number

Placement of the numeric page number derived from outline order. It is always shown, including on cover and conclusion pages; none is not allowed.

#### oneOf[0].content.decorations.section_title

Placement of the current top-level section title from the outline, not the subsection title. Use none to hide it; missing text is not rendered.

#### oneOf[0].content.demands

Visual design requirements that page authoring must follow throughout this presentation. Record independently actionable natural-language requirements covering overall look, information density, layout and hierarchy, text/image expression, use of visual elements, reference and brand constraints, or motion. Items may include conditions, preferences or explicit prohibitions; do not require every dimension. Record actual project requirements only, not generic quality rules or copies of the selected theme's existing configuration. Keep content requirements in Manifest and a single page's arrangement in its Spec. Explicit palette or typography requirements must be fulfilled through available theme capabilities; explain unsupported requests. Changing this list updates authoring guidance, not existing HTML. Supplying it replaces the list completely; use [] when unspecified or to clear it.

#### oneOf[0].content.demands[]

One independent, explicit and actionable deck-wide visual design requirement, optionally with conditions, a preference or a prohibition.

#### oneOf[1]

The user refused this operation. It was not applied; no edit was committed or command executed.

#### oneOf[1].decision

The user declined this operation; this is a normal decision, not a tool error.

#### oneOf[1].feedback

Optional verbatim user reason or suggestions. Use it to revise the approach within authorization; do not repeat the denied intent unchanged or treat feedback as approval.

#### oneOf[1].summary

Acknowledgement that the operation was refused and did not take effect.

## edit_spec

### 工具描述

Edit supplied top-level fields; omitted fields remain unchanged, arrays replace completely. Requires slide_id. First creation requires only core and elements. Set purpose, content_type or layout to null to remove that optional field; null is never saved. Changing purpose away from content or removing it also clears an existing content_type. Validate content_type against the merged complete Spec, not just this call's fields.

### 输入参数

#### content_type

Optional dominant communication mode of a body page: explanation explains concepts, definitions, principles or facts; comparison compares objects, differences and trade-offs; example illustrates through concrete cases or examples; guidance provides methods, steps or action recommendations; other means a known body-content mode outside these categories, such as an exercise or task. Allowed only when the saved purpose is content. Omit for mixed content or an undetermined mode; never default to other, and do not use other for unknown or insufficient information. Omit to keep the current value; pass null to remove it. The merged saved purpose must be content; it may be retained from the existing Spec when purpose is omitted from this call.

#### content_type.anyOf[0]

Optional dominant communication mode of a body page: explanation explains concepts, definitions, principles or facts; comparison compares objects, differences and trade-offs; example illustrates through concrete cases or examples; guidance provides methods, steps or action recommendations; other means a known body-content mode outside these categories, such as an exercise or task. Allowed only when the saved purpose is content. Omit for mixed content or an undetermined mode; never default to other, and do not use other for unknown or insufficient information.

#### core

The main takeaway the audience should retain from this page, not its title or a list of layout instructions.

#### elements

Ordered content elements planned for this page. Supplying this list replaces it completely; [] clears it. Each element describes what to communicate and how to represent it, not HTML source.

#### elements[]

One planned content element, pairing a representation type with its communication intent.

#### elements[].intent

What this element should communicate: the relevant content, evidence or relationship and its purpose on the page. Describe the intended content in natural language without inventing missing facts.

#### elements[].type

Representation of this element: text for prose, list for items, metric for a key figure, quote for a quotation, table for tabular values, chart for data, diagram for relationships, code for a code sample, or asset for visual media. comparison is a page content_type; choose chart, table or diagram for a comparison element.

#### layout

Natural-language suggestion for arranging this page's content and reading order, such as a chart on the left and explanation on the right; not a template identifier or CSS. Omit to keep the current value; pass null to remove it.

#### layout.anyOf[0]

Natural-language suggestion for arranging this page's content and reading order, such as a chart on the left and explanation on the right; not a template identifier or CSS.

#### purpose

Optional purpose of this page in the narrative: cover presents the topic and identity; introduction introduces the topic, background, problem, goals or structure; transition switches chapters or topics; content develops the body; conclusion closes with a summary, conclusions or action recommendations; other means a known purpose outside these categories. This is not a visual template. Omit when undetermined; never default to content or other. A saved content_type is allowed only when purpose is content. Omit to keep the current value; pass null to remove it. Setting a non-content purpose or null also clears an existing content_type in the same save. Do not supply a non-null content_type with that change.

#### purpose.anyOf[0]

Optional purpose of this page in the narrative: cover presents the topic and identity; introduction introduces the topic, background, problem, goals or structure; transition switches chapters or topics; content develops the body; conclusion closes with a summary, conclusions or action recommendations; other means a known purpose outside these categories. This is not a visual template. Omit when undetermined; never default to content or other. A saved content_type is allowed only when purpose is content.

#### slide_id

Stable ID of the existing Outline page to edit, within the authorized page scope. Pass the page's id as slide_id, not its title or page number; this tool does not create a page in the outline.

### 输出参数

Saved resource; omitted input fields are included with their retained values.

#### changed

Whether this call actually changed saved content. False means no repair was applied; do not treat it as a fixed defect or repeat the same edit.

#### changed_fields

Top-level fields whose stored values changed; empty for a no-op.

#### changed_fields[]

Changed field name in content.

#### content

Complete saved spec JSON object.

#### content.content_type

Optional dominant communication mode of a body page: explanation explains concepts, definitions, principles or facts; comparison compares objects, differences and trade-offs; example illustrates through concrete cases or examples; guidance provides methods, steps or action recommendations; other means a known body-content mode outside these categories, such as an exercise or task. Allowed only when the saved purpose is content. Omit for mixed content or an undetermined mode; never default to other, and do not use other for unknown or insufficient information.

#### content.core

The main takeaway the audience should retain from this page, not its title or a list of layout instructions.

#### content.elements

Ordered content elements planned for this page; an empty array means no elements are specified. Each item pairs representation and communication intent, not HTML.

#### content.elements[]

One planned content element, pairing a representation type with its communication intent.

#### content.elements[].intent

What this element should communicate: the relevant content, evidence or relationship and its purpose on the page. Describe the intended content in natural language without inventing missing facts.

#### content.elements[].type

Representation of this element: text for prose, list for items, metric for a key figure, quote for a quotation, table for tabular values, chart for data, diagram for relationships, code for a code sample, or asset for visual media. comparison is a page content_type; choose chart, table or diagram for a comparison element.

#### content.layout

Natural-language suggestion for arranging this page's content and reading order, such as a chart on the left and explanation on the right; not a template identifier or CSS.

#### content.purpose

Optional purpose of this page in the narrative: cover presents the topic and identity; introduction introduces the topic, background, problem, goals or structure; transition switches chapters or topics; content develops the body; conclusion closes with a summary, conclusions or action recommendations; other means a known purpose outside these categories. This is not a visual template. Omit when undetermined; never default to content or other. A saved content_type is allowed only when purpose is content.

## edit_html

### 工具描述

Create/replace slide HTML with content or apply exact edits; supply exactly one. HTML saves verbatim. Write readable source with consistent indentation; preserve existing layout during local edits. A successful known write can be edited again without rereading if unchanged. Read first when source is unknown, stale or missing from context; do not reconstruct anchors from memory. Saving proves a write, not appearance; call render_slide.

### 输入参数

#### content

Complete HTML source for this existing outline page. Creates missing HTML or replaces all existing HTML; not a fragment, file path or Markdown code fence. Mutually exclusive with edits. Follow the slide HTML contract and render after saving.

#### edits

Ordered exact replacements on currently known saved HTML, mutually exclusive with content. Each old_text must match once, including whitespace; each replacement sees earlier replacements, and the whole batch saves atomically. After an anchor mismatch, read the current HTML before constructing a new edit. Identical replacements apply no repair; do not submit them to refresh evidence.

#### edits[].new_text

Literal replacement source text. Use an empty string to delete the matched text; the final document must remain valid.

#### edits[].old_text

Exact non-empty source text to replace, including whitespace. It must occur exactly once when this replacement runs; this is not a regex or a diff.

#### slide_id

Stable ID of the existing Outline page to edit, within the authorized page scope. Pass the page's id as slide_id, not its title or page number; this tool does not create a page in the outline.

### 输出参数

HTML save acknowledgement; no source is echoed. Saving does not verify appearance; call render_slide.

#### changed

Whether the saved HTML actually changed. False means no repair was applied; inspect the current source before choosing a different necessary edit. A no-op alone does not require another render.

#### content_precheck

Optional advisory content precheck of the final saved page content in this tool batch. Present only for the successful call owning that page's final HTML change; omitted for unchanged HTML or an earlier edit superseded in the same batch. The original tool call identifies the page, so no slide_id is repeated. Scores do not verify visuals, factual truth or historical preservation; low scores and unavailable checks do not turn a successful save into a failed edit.

#### content_precheck.oneOf[0]

Completed scoring of the saved content; this is not a pass decision or task completion.

#### content_precheck.oneOf[0].scores

Independent content dimensions; use each field's rubric to interpret its score. A low score suggests checking the page, without identifying a specific defect.

#### content_precheck.oneOf[0].scores.content_coverage

Whether the current page contains the central message and required supporting material needed to perform its intended role. Expected score from 0 to 3, possibly fractional; higher is better. A fractional score is a probability-weighted mean, not an exact rubric level or a concrete defect finding. 0: The actual page content is absent or unrelated to the page's required purpose. 1: Some required points appear but the central message or major required material is missing. 2: The central message and most required material are present, with specific remaining omissions. 3: The actual content fully performs this page's intended role under the user requirements.

#### content_precheck.oneOf[0].scores.expression_clarity

Whether the current page's main message, supporting organization and wording are understandable and clear. Expected score from 0 to 3, possibly fractional; higher is better. A fractional score is a probability-weighted mean, not an exact rubric level or a concrete defect finding. 0: The text has no understandable message or meaningful organization. 1: The intended message can only be guessed because the structure or wording is confusing. 2: The main message is clear, with limited ambiguity or unnecessary complexity. 3: The conclusion, supporting organization and wording are consistently clear and understandable.

#### content_precheck.oneOf[0].scores.requirement_adherence

Whether the current page content follows the effective user requirements and restrictions, with original instructions and later corrections taking precedence over editable project resources. Expected score from 0 to 3, possibly fractional; higher is better. A fractional score is a probability-weighted mean, not an exact rubric level or a concrete defect finding. 0: The current content contradicts a key user requirement or restriction. 1: The current content has material deviations from the effective user requirements. 2: The current content follows the core requirements with minor deviations. 3: The current content follows the effective user requirements and restrictions.

#### content_precheck.oneOf[0].status

All three content scores are available, including when they are low.

#### content_precheck.oneOf[1]

No usable scores; the HTML save remains successful.

#### content_precheck.oneOf[1].reason

Reason code for missing or invalidated scores, e.g. timeout, invalid_answer, material_too_large, disabled or material_changed. This describes precheck availability, not a page defect.

#### content_precheck.oneOf[1].status

unavailable: evaluation could not provide usable scores; skipped: not evaluated, e.g. scoring is disabled; stale: evaluated material changed and the assessment no longer applies.

#### summary

HTML save acknowledgement; no source is echoed. Saving does not verify appearance; call render_slide.

## edit_outline

### 工具描述

Initialize an absent outline with init, or edit existing source using edits. Supply exactly one. Sections require title, purpose, slides and subsections; subsections require title, purpose and slides. A new slide contains only title (no id or purpose). Existing slides contain only id and title. Every node uses id for its own stable identity; slide IDs have the sli_ prefix. Omit id for all new nodes so Runtime assigns it; preserve existing identities when moving or editing. Pass the page's Outline id as slide_id when calling page tools. Use either direct slides or subsections per section, never both. Runtime assigns new identities. Existing identities must be preserved. Removed pages delete their Spec and HTML atomically. Creation and edits require user approval before saving.

### 输入参数

#### edits

Ordered exact text replacements on existing outline JSON source, mutually exclusive with init. Read the saved source first; each replacement sees earlier replacements, and the batch must produce a valid outline. Section, subsection and page id values are stable identities, not positions. Sections require title, purpose, slides and subsections; subsections require title, purpose and slides. A new slide contains only title (no id or purpose). Existing slides contain only id and title. Every node uses id for its own stable identity; slide IDs have the sli_ prefix. Omit id for all new nodes so Runtime assigns it; preserve existing identities when moving or editing. Pass the page's Outline id as slide_id when calling page tools. Use either direct slides or subsections per section, never both.

#### edits[].new_text

Literal replacement source text. Use an empty string to delete the matched text; the final document must remain valid.

#### edits[].old_text

Exact non-empty source text to replace, including whitespace. It must occur exactly once when this replacement runs; this is not a regex or a diff.

#### init

Complete initial outline object with a sections array, used only when no outline exists; mutually exclusive with edits. sections lists top-level narrative groups in order; each section's subsections lists its second-level groups, and slides lists ordered page entries. title is the user-visible heading of the section, subsection or page; purpose states what a section or subsection contributes to the narrative. Keep unused slides/subsections arrays empty. Sections require title, purpose, slides and subsections; subsections require title, purpose and slides. A new slide contains only title (no id or purpose). Existing slides contain only id and title. Every node uses id for its own stable identity; slide IDs have the sli_ prefix. Omit id for all new nodes so Runtime assigns it; preserve existing identities when moving or editing. Pass the page's Outline id as slide_id when calling page tools. Use either direct slides or subsections per section, never both.

### 输出参数

Normal reply: the requested operation completed, or the user refused it. A refusal does not apply the operation or satisfy calls that require its effect.

#### oneOf[0]

Saved outline including generated stable IDs.

#### oneOf[0].changed

Whether the outline actually changed. False means no repair was applied; do not change whitespace merely to refresh validation evidence.

#### oneOf[0].content

Exact saved outline JSON source string, not a parsed object; use directly for exact edits. sections contains ordered top-level groups with id, title, purpose, slides and subsections. Subsections have id, title, purpose and slides; each page has id and title. title is the visible heading of a section, subsection or page; purpose explains a section or subsection's contribution to the narrative. IDs are stable backend-assigned identities, not positions; page IDs have the sli_ prefix and are passed as slide_id to page tools. A section uses direct slides or subsections, not both; unused arrays are empty.

#### oneOf[1]

The user refused this operation. It was not applied; no edit was committed or command executed.

#### oneOf[1].decision

The user declined this operation; this is a normal decision, not a tool error.

#### oneOf[1].feedback

Optional verbatim user reason or suggestions. Use it to revise the approach within authorization; do not repeat the denied intent unchanged or treat feedback as approval.

#### oneOf[1].summary

Acknowledgement that the operation was refused and did not take effect.

## git_commit

### 工具描述

Commit the current project's durable source files to its local Git repository. Ground the title and summary items in the supplied staged changes; if no change evidence was supplied, inspect git status and git diff with run_command first. Includes all project-whitelisted source changes, not just the current slide scope; excludes caches, exports and thumbnails. Does not push. Use when the user requests a commit or a saved project version; do not commit after every edit. If a commit result is uncertain, inspect history and report it rather than issuing another commit. In /commit, call this tool exactly once without accompanying text; the host executes the commit and ends the command without another model turn.

### 输入参数

#### items

Commit body entries summarizing the inspected source changes. Runtime formats them as a bullet list; do not include future work.

#### items[]

One distinct actual change in one Chinese sentence, without a bullet prefix or unsupported claims of testing or completion.

#### title

Single-line Chinese title summarizing the actual source changes being committed: \<feat|fix|refactor|perf|chore|docs>: \<primary change>, without a trailing period or invented completion claims.

### 输出参数

Local commit outcome returned in the agent loop; no remote push occurs. In /commit, the host publishes the command result and ends execution without sending a tool reply to the model.

#### branch

Branch where the commit was created; omitted with hash when there were no changes.

#### hash

Git commit identifier, present only when a commit was created.

#### summary

Commit acknowledgement, or a message that there were no changes to commit.

## run_command

### 工具描述

Run a restricted project-local command against the current durable project state. Supported reads: pwd, ls, cat, head, tail, find, grep, rg, jq, stat, sed, wc, git status, git diff, and git log. Use | for stdout pipelines and && for sequential success; every executable is checked separately and no shell is invoked. Common read-only options support combined short flags, attached or separate values, multiple operands, --option=value, and --. All file operands, option-loaded files, and recursive symlink targets must stay inside the project; use relative paths without .. or absolute paths. Sensitive reads require user approval when high-risk operation approval is enabled. Redirections, ||, semicolons, substitutions, variables, unquoted shell globs, helper execution, and implicit writes are denied. Quote patterns. Examples: `grep -rn --include='*.html' color . | head -n50` and `rg -n -g '*.html' color .`. Tail follow, find -exec/-delete, rg --pre/--search-zip, jq module imports, and sed r/w/e are denied. Each pipeline has a 10 second timeout and bounded stdout/stderr. The only write form is a single-file sed -i substitution governed by high-risk operation approval in execute mode, excluding .manifest.json, .design.json, and .outline.json; use their focused edit tools so the separate resource approval setting applies. Read the full exact target text with single-file cat first; read_resource also supplies exact text for outline and HTML, but JSON objects do not authorize text edits. Command output is saved verbatim with domain validation, without formatting. A stale or unseen version is rejected.

### 输入参数

#### command

Project-local inspection command using pwd/ls/cat/head/tail/find/grep/rg/jq/stat/sed/wc or the restricted git status/diff/log forms. Only | and && may join commands, and each stage must be allowed. Use relative project paths, quoted patterns, normal combined flags and option values (e.g. grep -rn --include='*.html' color . | head -n50). Do not use redirections, ||, ;, shell expansion, helper execution, or options/scripts that read or write outside the checked project files. The only write form is sed -i '' 's/pattern/replacement/g' FILE, used alone in execute mode after a full current single-file cat read; Runtime requests approval when high-risk operation approval is enabled. sed scripts may select, print, delete from the output stream, or substitute in the output stream, but may not execute helpers or load/write files implicitly. Additional grep/rg/jq/sed option-loaded files are also checked; jq import/include and module paths are unavailable.

### 输出参数

Normal reply: the requested operation completed, or the user refused it. A refusal does not apply the operation or satisfy calls that require its effect.

#### oneOf[0]

Captured command stdout, stderr and exit status. No full-output file or continuation handle is provided.

#### oneOf[0].content_precheck

Optional advisory content precheck of the final saved page content in this tool batch. Present only for the successful call owning that page's final HTML change; omitted for unchanged HTML or an earlier edit superseded in the same batch. The original tool call identifies the page, so no slide_id is repeated. Scores do not verify visuals, factual truth or historical preservation; low scores and unavailable checks do not turn a successful save into a failed edit.

#### oneOf[0].content_precheck.oneOf[0]

Completed scoring of the saved content; this is not a pass decision or task completion.

#### oneOf[0].content_precheck.oneOf[0].scores

Independent content dimensions; use each field's rubric to interpret its score. A low score suggests checking the page, without identifying a specific defect.

#### oneOf[0].content_precheck.oneOf[0].scores.content_coverage

Whether the current page contains the central message and required supporting material needed to perform its intended role. Expected score from 0 to 3, possibly fractional; higher is better. A fractional score is a probability-weighted mean, not an exact rubric level or a concrete defect finding. 0: The actual page content is absent or unrelated to the page's required purpose. 1: Some required points appear but the central message or major required material is missing. 2: The central message and most required material are present, with specific remaining omissions. 3: The actual content fully performs this page's intended role under the user requirements.

#### oneOf[0].content_precheck.oneOf[0].scores.expression_clarity

Whether the current page's main message, supporting organization and wording are understandable and clear. Expected score from 0 to 3, possibly fractional; higher is better. A fractional score is a probability-weighted mean, not an exact rubric level or a concrete defect finding. 0: The text has no understandable message or meaningful organization. 1: The intended message can only be guessed because the structure or wording is confusing. 2: The main message is clear, with limited ambiguity or unnecessary complexity. 3: The conclusion, supporting organization and wording are consistently clear and understandable.

#### oneOf[0].content_precheck.oneOf[0].scores.requirement_adherence

Whether the current page content follows the effective user requirements and restrictions, with original instructions and later corrections taking precedence over editable project resources. Expected score from 0 to 3, possibly fractional; higher is better. A fractional score is a probability-weighted mean, not an exact rubric level or a concrete defect finding. 0: The current content contradicts a key user requirement or restriction. 1: The current content has material deviations from the effective user requirements. 2: The current content follows the core requirements with minor deviations. 3: The current content follows the effective user requirements and restrictions.

#### oneOf[0].content_precheck.oneOf[0].status

All three content scores are available, including when they are low.

#### oneOf[0].content_precheck.oneOf[1]

No usable scores; the HTML save remains successful.

#### oneOf[0].content_precheck.oneOf[1].reason

Reason code for missing or invalidated scores, e.g. timeout, invalid_answer, material_too_large, disabled or material_changed. This describes precheck availability, not a page defect.

#### oneOf[0].content_precheck.oneOf[1].status

unavailable: evaluation could not provide usable scores; skipped: not evaluated, e.g. scoring is disabled; stale: evaluated material changed and the assessment no longer applies.

#### oneOf[0].exit_code

Command exit status.

#### oneOf[0].stderr

Captured standard error; empty when none was captured.

#### oneOf[0].stdout

Captured standard output.

#### oneOf[1]

The user refused this operation. It was not applied; no edit was committed or command executed.

#### oneOf[1].decision

The user declined this operation; this is a normal decision, not a tool error.

#### oneOf[1].feedback

Optional verbatim user reason or suggestions. Use it to revise the approach within authorization; do not repeat the denied intent unchanged or treat feedback as approval.

#### oneOf[1].summary

Acknowledgement that the operation was refused and did not take effect.

## create_plan

### 工具描述

Create or fully replace an unapproved plan draft and wait for user approval.

### 输入参数

#### content

Complete Markdown plan for user approval, stating the intended outcome, scope and approach. When revising, provide the full replacement plan, not only the changes.

#### steps

Ordered execution steps for the complete proposed plan. Runtime assigns step IDs; approval freezes this list.

#### steps[].title

A concrete action or deliverable that the user can recognize in plan progress.

#### title

User-facing title summarizing the work proposed for approval.

### 输出参数

User plan decision returned through the original call after waiting. Refusal rejects the current proposal, not the planning task.

#### decision

approve freezes the approved plan and starts execution; refuse keeps the draft unapproved and requires a complete revised proposal via create_plan. Do not execute or finish solely because of refusal.

#### feedback

Optional verbatim user reason or revision suggestions accompanying the decision. Reassess known requirements when absent; ask only for blocking missing information. This text alone does not authorize execution.

#### summary

Approval notice, or instructions to revise the rejected proposal against the feedback and original request; explicitly indicates missing feedback when none was supplied.

## update_plan

### 工具描述

Update existing step statuses using the updates array. Use exact step IDs from the current plan. Approved content and step structure are locked. At most one step may be processing; completed steps cannot regress.

### 输入参数

#### updates

Status changes for existing steps in the approved active plan. Include each step at most once; omitted steps keep their status.

#### updates[].status

Actual execution state: pending for work not currently underway, processing for current work, completed for finished work, or failed for unsuccessful work. At most one step may be processing; completed steps cannot regress.

#### updates[].step_id

Exact step ID supplied in the current plan context, not its title or position.

### 输出参数

Acknowledgement that the requested step statuses were saved; does not return or replace the plan.

#### summary

Acknowledgement that the requested step statuses were saved; does not return or replace the plan.

## request_privilege

### 工具描述

Request an expansion of the editable page set. Runtime waits for user approval when high-risk operation approval is enabled; otherwise it grants only the requested additional pages directly. Wait for this call's result before editing those pages. Provide only additional slide IDs and explain why in page terms. Global resources and both Spec/HTML of authorized pages are already writable. This call must be the only call in the response.

### 输入参数

#### reason

User-facing explanation of which additional pages need changes and why those changes are needed for the requested task.

#### slide_ids

Existing pages to add to the editable scope, not a replacement for the current scope. Use IDs from the current outline.

#### slide_ids[]

Stable ID of a page whose Spec or HTML needs editing beyond the current authorization.

### 输出参数

Page authorization outcome returned through the original call. Refusal and revision are business decisions, not execution errors.

#### decision

approve grants the requested pages through user approval or automatically when high-risk operation approval is disabled (or confirms existing authorization); revise immediately authorizes all pages, including run-created pages, without resubmission; refuse keeps the previous scope.

#### feedback

Optional verbatim user reason or suggestions accompanying the decision. Continue within the effective scope; suggestions do not independently authorize more pages or repeating the denied request unchanged.

#### summary

Explanation of the effective page authorization.

## ask_user

### 工具描述

Ask one blocking group of atomic user questions needed for the user's actual task and pause this same loop until the user answers. Do not use for greetings, identity questions, or to solicit a task the user has not requested. Each item is either single-choice with 1-3 options, optionally allow_custom=true, or fill-in with no options. Do not merge multiple choices into one free-text question.

### 输入参数

#### questions

Questions to present together in order. Each question should ask for one decision or missing fact needed to continue the user's task.

#### questions[].allow_custom

Whether to allow a free-text answer alongside preset options; defaults to false when options exist. With no options, free-text input is always enabled regardless of this value.

#### questions[].options

One to three preset answers for a single-choice question. Omit or use [] for a free-text question.

#### questions[].options[].description

Explain this option's concrete meaning, effect or tradeoff so the user can choose; do not merely repeat its label.

#### questions[].options[].label

Concise, distinct answer text displayed as the option and returned as the answer when selected.

#### questions[].question

The complete question shown directly to the user, including enough context to answer. Must be phrased as an interrogative sentence and end with '?' or '？'. Do not use a category label, statement, or instruction.

#### questions[].reason

Briefly explain to the user why their answer is needed and which subsequent work or decision it will affect. Do not repeat the question or provide internal reasoning.

### 输出参数

User answers after this call resumes.

#### answers

One item per original question in the original order, including skipped questions.

#### answers[]

Answer paired with its original question.

#### answers[].answer

Selected option label, or the user's verbatim custom/free-text answer. A skipped question returns this string instead of null: 用户跳过了此问题，请结合已有信息自行判断；如有推荐选项，可优先采用，但不要将其视为用户明确选择。 Skipping is not explicit approval.

#### answers[].question

Original questions[].question text, not an internal question ID.

## review_task

### 工具描述

Ask the independent artifact reviewer to inspect current PPT artifacts and cumulative Run changes. Describe the target pages and review focus in demand. The backend prepares complete current evidence, resources, local diffs and screenshots before a single model assessment; the Reviewer can only submit_review. It does not review plans or final replies, edit artifacts, or finish the task. Use its reasons to decide the next action.

### 输入参数

#### demand

What artifacts should be reviewed, what to check, and any task-specific acceptance requirements. User requirements remain authoritative.

### 输出参数

Independent artifact assessment; does not grant user authorization or finish the main task.

#### decision

approve supports delivery; revise requires verification or revision; refuse identifies defects blocking delivery.

#### reasons

Non-empty findings for every decision, including the evidence supporting approval.

#### reasons[]

Concrete approval basis, uncertainty or revision needed, or confirmed defect blocking delivery.

## finish_task

### 工具描述

Submit the complete final user-facing response for the current chat, ask, or execute run. For greetings, identity questions, acknowledgments, or questions answerable from available context, call this tool directly in the first response with a brief answer, without preceding assistant text or other tools. No project work is required for ordinary conversation. Use this call alone. Ordinary assistant text is not a completion signal. Runtime checks the requested outcome and any required evidence before completion.

### 输入参数

#### message

The complete answer to the current request. A greeting or identity question usually needs only one sentence; do not add project summaries, capability lists or next steps unless requested or relevant. For actual work, report the result and relevant checks or limitations.

#### suggested_next_inputs

Optional drafts of follow-up requests the user can select to fill the input composer, then edit or send. Provide zero to three suggestions grounded in the current project, completed request, remaining useful work and actual product capabilities. Put the most natural and valuable continuation first; additional suggestions must offer meaningfully different directions. Prefer no suggestions over weak or invented ones. Omit or use [] for greetings, acknowledgments, identity questions and other self-contained exchanges with no useful continuation; do not turn these replies into task invitations in message. Suggestions are separate metadata and must not replace or shorten the complete answer in message.

#### suggested_next_inputs[]

One ready-to-send command in the user's language, at most 80 Unicode characters. Refer to slides by visible numbers or titles, not internal IDs. Use plain text without numbering, Markdown, tabs or line breaks. Do not include 'you can' framing, rationale, assistant promises or status labels.

### 输出参数

On success, Runtime finishes this run and publishes the submitted message as the final answer; there is no tool reply.

## submit_review

### 工具描述

Assess the backend-prepared evidence and submit the final artifact review. A valid submission ends this review without a reply. Call it as the only tool call in the submission response. Invalid submissions may receive error feedback and be corrected within the shared deadline, output, context and two-additional-request budget. Reasons are required for all outcomes, including approval. This does not finish the main task or change any artifact.

### 输入参数

#### decision

Artifact review outcome: approve when checks support delivery, revise when further verification or revision is needed, or refuse when confirmed defects block delivery. This is a review assessment, not user approval or main-task completion.

#### reasons

Concrete reasons in the user's language, required for every outcome. For approve explain what was verified and supports delivery; for revise explain what needs verification or revision; for refuse explain the confirmed defects blocking delivery. Report findings, not internal deliberation.

#### reasons[]

One concrete review finding with the relevant page, observed evidence and impact, or the specific uncertainty or revision to address.

### 输出参数

On success, the prepared-evidence Reviewer assessment ends without a tool reply. Its decision and reasons are delivered to the main agent through review_task; this does not finish the main task.

## rename_thread

### 工具描述

Submit rename or keep through the only tool call in this response, without ordinary text. A valid submission is accepted once and ends this naming task; rejected calls may receive feedback for correction within the runtime budget.

### 输入参数

#### action

Use rename when recent work has a clear main topic that the current title does not represent; use keep when the title still fits or evidence is insufficient. keep must omit title.

#### title

New conversation title, required only for rename and forbidden for keep. Describe the recent main task or stage in the conversation language using single-line plain text, without Markdown, status prefixes or a trailing period.

### 输出参数

The caller consumes a valid rename/keep submission and ends the command without a tool reply.

## compact_context

### 工具描述

Submit the durable working context for the same continuing task.

### 输入参数

#### content

Complete Markdown working summary for continuing the same task, with exactly five level-2 sections: 目标与意图, 已完成改动, 关键决策, 未决问题 and 下一步. Preserve explicit constraints, decisions, relevant page and attachment IDs, actual progress, evidence limits and remaining work; distinguish completed work from attempts or plans. Historical scope decisions are not new authorization.

#### title

Short single-line plain-text timeline title in the conversation language identifying the task, stage or key decision, without Markdown, command prefixes or a trailing period.

### 输出参数

A valid submission is consumed once and ends this command without an acknowledgement.

## polish_instruction

### 工具描述

Return a wording suggestion for the supplied draft; do not answer or execute it.

### 输入参数

#### content

The complete revised draft ready for the user to send, or the unchanged draft when no edit is needed. Preserve intent, language and scope, applying the supplied revision feedback. Do not answer or execute the draft, invent requirements, or add an explanation around it.

#### title

Short single-line plain-text title in the user's language describing the wording improvement, such as correcting typos or clarifying a reference. State that the draft is unchanged when no edit is needed; do not repeat the task or claim it is complete. No Markdown, HTML or trailing period.

### 输出参数

A valid submission is consumed once and ends this command without an acknowledgement.

## handoff_thread

### 工具描述

Submit the requested prompt exactly once per submission response. A valid submission ends this command without an acknowledgement; rejected output may receive runtime feedback for a bounded correction. This does not create a thread or start another Agent.

### 输入参数

#### content

Complete standalone Markdown brief addressed to the receiving Agent in the user's language. Preserve the goal, accepted decisions, actual progress, evidence limits, remaining work and next action. On revision, return the full replacement brief; do not invent unfinished work when the task is complete.

#### title

Short single-line plain-text timeline title in the user's language identifying the work or stage being handed over, without Markdown, command prefixes or a trailing period.

### 输出参数

A valid submission is consumed once and ends this command without an acknowledgement.
