## load_component

Load a component's complete HTML source into `active_components` for reference during the current run. Do not reload a component that is already active. A successful call returns only `status=success`; the source is supplied through context.

Args：
- `component_id`: The component's unique ID from `available_components`, rather than its name or file path.

## load_skill

Load a skill's complete instructions into `active_skills` to guide the current run. Do not reload a skill that is already active. A successful call returns only `status=success`; the instructions are supplied through context.

Args：
- `skill_id`: The skill's unique ID from `available_skills`, rather than its name or file path.

## read_resource

Read the complete current saved source of a presentation resource. Use `project_state` to reuse fresh content, reread stale content when needed, and avoid reading missing resources. Reread an affected resource when a content conflict requires a new baseline. Provide `slide_id` for `spec` and `html`; omit it for other resource types. Returns `content` as a string containing the original JSON or HTML source.

Args：
- `slide_id`: Stable unique slide ID from the current outline.
- `type`: `manifest` for global content requirements; `design` for global visual requirements; `outline` for the presentation's sections, subsections, and slides; `spec` for semantic content plan; `html` for exact source.

## read_image

View an uploaded image or the latest screenshot of a slide. Supply exactly one of `attachment_id` or `slide_id`; the requested image is returned directly for visual inspection. Render the slide first if its screenshot is missing or stale. `IMAGE_INVALID` indicates an unusable image, including empty, damaged, unsupported or stale image content.

Args：
- `attachment_id`: Uploaded unique image ID from attachment context, rather than a file path or URL.
- `slide_id`: Stable unique slide ID from the current outline, rather than a file path or URL.

## render_slide

Render saved slide HTML into a screenshot and report rendering problems. Use it to inspect newly authored or modified slides and verify fixes. If the render is fresh and only viewing is needed, reuse the screenshot from context or call `read_image`.

Set `assess_quality` to `true` to request an independent assessment of content coverage, expression clarity and compliance with applicable requirements. Successful rendering does not establish slide quality. Assessment may be unavailable even when rendering succeeds; an absent `preview` is not a passing judgment. Verify assessment findings against the screenshot and requirements before making changes.

When a before-edit screenshot is available in context, compare it with the latest render to check the intended change, preserved content and regressions. Historical screenshots do not represent the current saved sources.

The result includes a screenshot and `diagnostics`:

- `overflow`: Horizontal or vertical content overflow beyond the canvas, including dimensions and excess pixels.
- `console_errors`: Browser console errors or uncaught script errors.
- `failed_resources`: Failed requests, HTTP 400+ responses or blocked external resources, deduplicated.
- `out_of_bounds`: Elements extending beyond `.slide-stage`, or the document root if absent. Each entry includes `tag`, optional `id` and `class`, and `rect` edges in viewport CSS pixels rounded to integers.
- `text_clipping`: Text clipped by a container, including `element`, `container`, affected `text`, `directions`, `clipped_px` and `clip_rect` in viewport CSS pixels. Decorative cropping without text is excluded.

An available `preview` contains advisory assessment results:

- `scores`: Independent 0–3 scores for `content_coverage`, `expression_clarity` and `requirement_adherence`; higher is better. Fractional scores are probability-weighted rubric means, not precise defect findings or an overall pass score.
- `judges.mismatch`: The primary explicit agreement mismatch: `manifest_mismatch` for global content requirements, `spec_mismatch` for slide content or composition, `design_mismatch` for visual requirements, or `none`.
- `judges.issue`: The primary observed issue: `expression_issue` for logic or wording, `layout_issue` for spatial arrangement or clipping, `visual_issue` for typography, color or hierarchy, or `none`.

`RENDER_FAILED` indicates an authoring violation or blocking diagnostics: overflow, out-of-bounds elements, text clipping, console errors or failed resources.

Args：
- `assess_quality`: `true` requests an additional slide quality assessment; `false` only returns the screenshot and diagnostics.
- `slide_id`: Stable unique slide ID from the current outline.

## edit_manifest

Create or update presentation-wide content requirements. Use this for global goals, audience and restrictions rather than a change limited to one slide. Omitted fields retain their saved values. Supplied arrays replace the entire saved arrays, so include every entry that should remain. Returns the complete saved manifest JSON object in `content`, including any user revisions during approval. A refusal saves no changes and may include the user's reason in `feedback`.

Args：
- `audience`: A concise description of the presentation's target audience, guiding content depth and terminology complexity.
- `goal`: A concise statement of the presentation's purpose, focusing on what the audience should understand or be able to do.
- `language`: The language used in the presentation, specified as a language tag such as `en-US` or `zh-CN`.
- `slide_count`: The desired number of slides in the presentation, which may be a range or an approximation.
- `prohibitions`: Each item specifies a concrete content restriction tailored to this presentation and shared across slides.
- `requirements`: Each item specifies a concrete content requirement tailored to this presentation and shared across slides.
- `title`: A concise title for the entire presentation that captures its main theme, scope, and content.

## edit_design

Create or update presentation-wide visual requirements. Express requirements and prohibitions in natural language, without frontend code or theme implementation instructions. Omitted fields retain their saved values. Supplied arrays replace the entire saved arrays, so include every entry that should remain. Returns the complete saved design JSON object in `content`, including any user revisions during approval. A refusal saves no changes and may include the user's reason in `feedback`.

Args：
- `prohibitions`: Each item specifies a concrete visual restriction tailored to this presentation and shared across slides.
- `requirements`: Each item specifies a concrete visual requirement tailored to this presentation and shared across slides.

## edit_spec

Create or update the content and composition plan of an existing slide. Provide `core`, `purpose` and `layout` when creating a spec. When updating, omitted fields retain their saved values. These fields must remain nonempty and cannot be removed or set to `null`. Supplied `elements` replace the complete saved list, so include every entry that should remain. A successful call returns the complete saved spec as a JSON object in `content`.

Args：
- `core`: A concise statement of the main takeaway the audience should retain from this slide, rather than its title, a content inventory or layout instructions.
- `elements`: The complete ordered list of semantic content units for this slide; order follows how the audience should read them.
- `elements.intent`: A concise statement of the concrete message, facts, evidence or relationship this element should convey and why it belongs on this slide.
- `elements.type`: `text` for prose; `list` for items; `metric` for a key figure; `quote` for a quotation; `table` for tabular values; `chart` for data visualization; `diagram` for relationships; `code` for a code sample; `asset` for visual media.
- `layout`: A concise statement of how content is spatially arranged to improve visual appeal and readability, rather than specific HTML or CSS implementation details.
- `purpose`: `cover` for topic and identity; `introduction` for background, problem, goals or structure; `transition` for a chapter change; `content_explanation` for concepts or facts; `content_comparison` for differences or tradeoffs; `content_example` for concrete cases; `content_guidance` for methods or recommendations; `content_other` for a known body-content mode outside these categories; `conclusion` for summary, conclusions or actions; `other` for a known page role outside these categories. Choose the dominant purpose for mixed content.
- `slide_id`: Stable unique slide ID from the current outline.

## edit_outline

Create or update the presentation's sections, subsections and slides. Supply exactly one of `content` for a complete outline or `edits` for targeted replacements.

- The root contains an ordered `sections` array. A section may contain direct `slides` or `subsections`, but not both; use `[]` for unused child arrays.
- Set each new node's `id` to `"new"`. Each occurrence receives a distinct stable ID; use returned IDs in subsequent calls.
- Preserve existing IDs when renaming, moving or reordering nodes. Do not duplicate IDs or change their node types.
- Existing slides omitted from the final outline are deleted along with their saved spec and HTML.
- Replacements run in order. Each `old_text` must match exactly once in the JSON after earlier replacements.

Returns the complete saved outline JSON source as a string in `content`, including assigned IDs and any user revisions during approval. A refusal may include the user's reason in `feedback`.

An outline can contain both kinds of section:

```json
{
  "sections": [
    {
      "id": "new", "title": "Introduction",
      "slides": [{ "id": "new", "title": "Overview" }],
      "subsections": []
    },
    {
      "id": "new", "title": "Analysis", "slides": [],
      "subsections": [{
        "id": "new", "title": "Key findings",
        "slides": [{ "id": "new", "title": "Supporting evidence" }]
      }]
    }
  ]
}
```

Args：
- `content`: Complete outline JSON source to create or fully replace the outline, without Markdown fences.
- `edits`: Ordered exact text replacements in the current saved outline JSON source.
- `edits.new_text`: Literal replacement source; use an empty string to delete the matched text.
- `edits.old_text`: Exact, non-empty source text to replace, including whitespace and enough surrounding context to identify one occurrence.

## edit_html

Create or update the HTML of an existing slide. Supply exactly one of `content` for creation or a complete rewrite, or `edits` for localized changes. Replacements run in order. Each `old_text` must match exactly once, including whitespace, in the HTML after earlier replacements.

Args：
- `content`: The complete HTML document to create or replace, without Markdown fences.
- `edits`: The ordered list of exact text replacements to apply to the current saved HTML.
- `edits.new_text`: The literal replacement source; use an empty string to delete the matched text.
- `edits.old_text`: The exact, non-empty source text to replace, including whitespace.
- `slide_id`: Stable unique slide ID from the current outline.

## create_plan

Create or replace an unapproved plan and submit it for user approval. Use it when a task needs explicit steps and dependencies before execution, in either Plan or Create mode. Keep the plan's rationale and intended outcomes in `content`; put the complete ordered execution steps only in `steps`. The approval view displays those steps separately, so do not repeat them in the Markdown body. Resubmit the complete proposal and step list when revising an unapproved plan.

Rejection or revision requests are returned in `feedback`. An approved plan's structure is locked; use `update_plan` to record execution progress. `PLAN_INVALID` indicates a conflict with the current plan state.

Args：
- `content`: Markdown explanation of the request, implementation approach, relevant constraints and expected outcomes. Do not include the execution step list.
- `steps`: Complete ordered execution steps, each a concise sentence describing one action or deliverable. This is the sole source for approval and progress displays.
- `title`: User-facing title of the proposed work.

## update_plan

Update execution statuses of existing steps in the approved active plan. Read `current_plan` and report actual progress, marking steps completed only after their required outcomes are achieved. Omitted steps retain their status. Batch completed steps in one call and omit steps already marked `completed`. At most one step may be `processing`; completed steps cannot revert to another status. `PLAN_INVALID` indicates an invalid plan state or progress update.

Args：
- `updates`: Status changes for existing approved steps.
- `updates.status`: `pending` for work awaiting execution; `processing` for work in progress; `completed` for finished work that achieved its expected result; `failed` for work whose execution failed.
- `updates.step_id`: Exact unique step ID from the current plan context.

## request_privilege

Request authorization for additional slides when the current scope is insufficient to perform required edits or renders. Submit this call alone and wait for the result. Use the updated `run_state` scope after approval. A refusal leaves the scope unchanged and may include the user's reason in `feedback`.

Args：
- `reason`: User-facing explanation of why the authorized slide scope needs to be expanded.
- `slide_ids`: IDs of the slides to add to the current authorized scope.

## ask_user

Ask the user for essential information or a decision that materially affects the task's direction or outcome and cannot be resolved from available context. Submit this call alone and wait for the result. Batch only independently answerable questions; defer dependent questions until their prerequisites are answered. Offer meaningful alternatives rather than filling an option quota.

Returns `answers` in the submitted question order. An answered question returns `{"state":"answered","text":"..."}`, preserving the selected option's original label or the user's verbatim response in `text`. A skipped question returns only `{"state":"skipped"}`. A skip is not a choice or approval; use available information to decide how to proceed.

Args：
- `questions`: Ordered questions to present together, each addressing one fact or decision.
- `questions.options`: One to three preset answers for a single-choice question; custom text is always available. Omit this field or pass `[]` to show a text input instead.
- `questions.options.description`: Concrete meaning, effect or tradeoff of this option.
- `questions.options.label`: Concise and distinct option text. If you strongly recommend this option, append `(Recommended)` to its end.
- `questions.question`: Complete user-visible interrogative sentence, ending with question mark.
- `questions.reason`: Brief user-visible explanation of why the answer is needed and what subsequent work it affects.

## review_ppt

Request an independent assessment of the current presentation and cumulative changes in this run. Use it after substantial authoring or revisions, for complex requirements, or when the user explicitly requests a review. Submit this call alone and wait for the result. Returns `decision` and a non-empty `reasons` list with supporting findings:

- `approve`: Sufficient evidence that the reviewed scope meets the requirements.
- `revise`: Important uncertainty requiring further verification.
- `refuse`: Confirmed defects that prevent delivery.

`REVIEW_FAILED` indicates unavailable or unusable review evidence, or an invalid review response.

Args：
- `demand`: Self-contained instructions for the reviewer, including the review objective, key areas to focus on, acceptance criteria, relevant user requirements and decisions, and the feedback you need.

## suggest_request

Submit useful next requests for the user to choose from. Call exactly once, without other tools or ordinary assistant text. Provide zero to three distinct, ready-to-send requests in the user's language, ordered by usefulness. Use an empty array for greetings, acknowledgments, identity questions or when no useful continuation is supported. Submission does not execute the suggested requests.

Args：
- `requests`: Zero to three distinct single-line plain-text requests the user is likely to make next, inferred from the conversation, current project and completed work. Each is a self-contained ready-to-send sentence matching the user's language and wording.

## git_commit

Submit a commit message for the staged project changes.

Args：
- `items`: Distinct body entries, each summarizing one coherent change in a single sentence.
- `title`: Concise single-line plain-text title that summarizes the primary change, in the format `<feat|fix|refactor|perf|chore|docs>: <primary change>`, without Markdown, HTML, surrounding quotation marks, status prefixes, or a trailing period.

## run_command

Run shell commands for batch processing, search or operations not covered by dedicated tools. Prefer resource editing tools for structured presentation changes and localized slide HTML edits.

Commands run in a disposable project copy through sandbox-runtime. Use project-relative paths for changes and modify only project content files; do not write elsewhere, alter application internals or perform destructive system operations. Chat, Ask and Plan are read-only; only Create execution can prepare project writes for approval.

Read only the project copy, system toolchain and configured read roots. Network access is disabled unless the host configuration allows the destination; local services remain blocked. The default deadline is 120 seconds and the combined stdout/stderr limit is 8 MiB. Do not bypass these limits. Refusal, failure, cancellation or timeout prevents local changes from being saved.

Returns `exit_code`, `stdout` and `stderr`; a refusal may include the user's reason in `feedback`. Command-specific errors are:

- `COMMAND_INVALID`: Empty command or invalid shell syntax.
- `COMMAND_NOT_ALLOWED`: Prohibited write or operation.
- `COMMAND_NOT_AVAILABLE`: Required executable or shell sandbox is unavailable.
- `COMMAND_EXIT_NONZERO`: Unsuccessful shell exit.
- `COMMAND_TIMEOUT`: Execution deadline exceeded.
- `COMMAND_OUTPUT_LIMIT`: Output limit exceeded; execution terminated.

Args：
- `command`: Bash source to execute from the project directory.

## rename_thread

Submit a decision to rename the conversation or keep its current title.

Args：
- `action`: `rename` for a missing title or one that no longer represents the recent main topic; `keep` for a title that still fits or when the available information is insufficient to choose a better one.
- `title`: Concise single-line plain-text title that identifies the conversation's main topic or current stage of work, without Markdown, HTML, surrounding quotation marks, status prefixes, or a trailing period.

## compact_context

Submit a concise working summary that preserves the context needed to continue the same task.

Args：
- `content`: Complete Markdown working summary for continuing the same task.
- `title`: Concise single-line plain-text title that identifies the task, stage, or key decision captured in the summary, without Markdown, HTML, surrounding quotation marks, status prefixes, or a trailing period.

## polish_instruction

Submit an improved version of the user's draft while preserving its intended meaning.

Args：
- `content`: Complete revised draft ready to send, or the original draft if no changes are needed.
- `title`: Concise single-line plain-text title that describes the wording improvement or indicates that the draft is unchanged, without Markdown, HTML, surrounding quotation marks, status prefixes, or a trailing period.

## handoff_thread

Submit a self-contained handoff brief for another conversation to continue the user's work.

Args：
- `content`: Complete standalone Markdown handoff brief, ready to send as the first message to the receiving Agent.
- `title`: Concise single-line plain-text title that identifies the work or stage being handed over, without Markdown, HTML, surrounding quotation marks, status prefixes, or a trailing period.
