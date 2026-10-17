## load_component

**Load component HTML sources as references for constructing slides**.
Cases:
- You need to reuse an existing HTML component or read it as a reference.
Rules:
- A successful call adds the complete source to `active_components` for the current run and returns only `status=success`.
- Do not load a component again if it is already present in `active_components`.
Args:
- `component_id`: The component's unique ID from `available_components`, rather than its name or file path.

## load_skill

**Load skill instructions to guide presentation creation**.
Cases:
- You need to consult an existing skill's workflows, methods, or guidelines to improve the quality of your work.
Rules:
- A successful call adds the complete skill instructions to `active_skills` for the current run and returns only `status=success`.
- Do not load a skill again if it is already present in `active_skills`.
Args:
- `skill_id`: The skill's unique ID from `available_skills`, rather than its name or file path.

## read_resource

**Read the complete current saved content of a presentation resource**.
Cases:
- You need the latest saved content to analyze the presentation or prepare an edit.
- Previously read content is stale, or an edit reports a content conflict that requires rereading the affected resource.
Rules:
- Use `project_state` to decide whether to read a resource: reuse `fresh` content, read `stale` resources when needed, and do not read resources marked `missing`.
- `slide_id` is required when `type` is `spec` or `html` and must be omitted for all other resource types.
Returns:
- `content`: Complete current saved source as a string: original JSON source for manifest, design, outline and spec; original HTML source for html.
Args:
- `slide_id`: Stable unique slide ID from the current outline.
- `type`: `manifest` for global content requirements; `design` for global visual requirements; `outline` for the presentation's sections, subsections, and slides; `spec` for semantic content plan; `html` for exact source.

## read_image

**View an uploaded image or the latest screenshot of a slide**.
Cases:
- Inspect an uploaded image to understand what the user is referring to and what they want to achieve.
- Inspect a slide screenshot to check whether the HTML renders as intended.
Rules:
- Supply exactly one of `attachment_id` or `slide_id`. Do not provide both in the same call.
- A successful call returns the requested image directly for visual inspection.
Errors:
- `IMAGE_INVALID`: The image exists but is empty, damaged or unsupported.
Args:
- `attachment_id`: Uploaded unique image ID from attachment context, rather than a file path or URL.
- `slide_id`: Stable unique slide ID from the current outline, rather than a file path or URL.

## render_slide

**Render the saved slide HTML into a screenshot, report rendering problems, and optionally assess slide quality**.
Cases:
- Inspect how a newly created or modified slide actually renders and whether it meets the user's requirements.
- Investigate a reported layout or rendering problem, or verify that a fix worked.
- You need additional feedback on a slide's content coverage, expression clarity, or adherence to requirements.
Rules:
- If the slide's `render` state in `project_state` is `fresh` and you only need to view the screenshot, reuse it from context or use `read_image`.
- Set `need_preview` to `true` when an additional quality assessment would help and is really necessary.
- Successful rendering alone does not establish that the slide meets the requirements.
- Compare `previous` and `current` screenshots of the same slide against the intended edit: verify the target defect is fixed, required content is preserved, and no regression was introduced. `previous` is historical comparison only; only `current` reflects the saved sources. If no current screenshot is available after an edit, render first.
- Treat `preview` as advisory feedback on the current slide. Verify indicated problems against the rendered slide and current requirements before making changes.
Returns:
- `diagnostics`: Observed rendering problems.
- `diagnostics.overflow`: Horizontal and vertical overflow of scrollable content beyond the canvas viewport, with observed dimensions and excess pixels.
- `diagnostics.console_errors`: Browser console errors or uncaught script errors.
- `diagnostics.failed_resources`: Deduplicated failed requests, HTTP 400+ responses or blocked external resources.
- `diagnostics.out_of_bounds`: Descendants extending beyond .slide-stage (document root if absent). Entries contain `tag`, optional `id`/`class`, and `rect` with `left`/`top`/`right`/`bottom` edges in viewport CSS pixels rounded to integers.
- `diagnostics.text_clipping`: Text cut off by a clipping container, even inside the canvas. Each entry identifies `element`, `container`, affected `text`, clipping `directions`, `clipped_px`, and `clip_rect` in viewport CSS pixels. Image and decorative cropping without text is excluded.
- `preview`: Optional present assessment.
- `preview.scores`: Three independent scores from 0 to 3, higher is better; fractional values are probability-weighted means of rubric levels, not exact defect findings or an overall pass score.
- `preview.scores.content_coverage`: Coverage of this slide’s intended message and required support. `0`: absent or unrelated; `1`: central message or major material missing; `2`: central message and most material present with specific omissions; `3`: fully performs this slide’s intended role.
- `preview.scores.expression_clarity`: Clarity of the main message, organization and wording. `0`: no understandable message or organization; `1`: meaning must be guessed; `2`: clear message with limited ambiguity or complexity; `3`: consistently clear conclusion, supporting organization and wording.
- `preview.scores.requirement_adherence`: Compliance with the requirements and restrictions in Manifest, Design, and Spec that apply to this slide. `0`: contradicts a key requirement; `1`: material deviations; `2`: core requirements met with minor deviations; `3`: fully follows applicable requirements and restrictions.
- `preview.judges`: Two independent single-choice judgments; one primary mismatch source and one primary slide issue.
- `preview.judges.mismatch`: `manifest_mismatch`: violates applicable global content, audience, language, goals or content restrictions; `spec_mismatch`: misses explicit core, content, approach or layout planning; `design_mismatch`: violates explicit visual requirements or prohibitions; `none`: none of these resource mismatches was observed.
- `preview.judges.issue`: `expression_issue`: unclear logic, organization, wording or excessive redundancy; `layout_issue`: overlap, cropping, misalignment, spatial grouping or position-based reading order obstructs reading; `visual_issue`: problematic color, typography, contrast, emphasis, hierarchy or style beyond concrete spatial errors; `none`: none of these slide issues was observed.
Errors:
- `RENDER_FAILED`: The slide's HTML violates authoring requirements or its observed rendering diagnostics report overflow, out-of-bounds elements, container text clipping, console errors or failed resources.
Args:
- `need_preview`: `true` requests an additional slide quality assessment; `false` only returns the screenshot and diagnostics.
- `slide_id`: Stable unique slide ID from the current outline.

## edit_manifest

**Create or update presentation-wide content demands**.
Cases:
- Establish the overall content of the presentation before authoring the slides.
- The user explicitly requests changes to presentation-wide content requirements rather than requirements for an individual slide.
Rules:
- Omitted fields retain their saved values.
- When updating an array, provide all entries that should be present after the update, including existing entries you want to retain.
- `status=refuse` means the user rejected the proposed edit and no changes were saved.
Returns:
- `content`: The complete saved manifest JSON object, including any user revisions during approval.
- `feedback`: The user's reason for rejection.
Args:
- `audience`: A concise description of the presentation's target audience, guiding content depth and terminology complexity.
- `goal`: A concise statement of the presentation's purpose, focusing on what the audience should understand or be able to do.
- `language`: The language used in the presentation, specified as a language tag such as `en-US` or `zh-CN`.
- `slide_count`: The desired number of slides in the presentation, which may be a range or an approximation.
- `prohibitions`: Each item specifies a concrete content restriction tailored to this presentation and shared across slides.
- `requirements`: Each item specifies a concrete content requirement tailored to this presentation and shared across slides.
- `title`: A concise title for the entire presentation that captures its main theme, scope, and content.

## edit_design

**Create or update presentation-wide visual demands**.
Cases:
- Establish the overall visual direction of the presentation before authoring the slides.
- The user explicitly requests changes to presentation-wide visual requirements rather than requirements for an individual slide.
Rules:
- Omitted fields retain their saved values.
- When updating an array, provide all entries that should be present after the update, including existing entries you want to retain.
- `status=refuse` means the user rejected the proposed edit and no changes were saved.
- Describe visual requirements and restrictions in natural language. Do not include frontend code or instructions for using themes.
Returns:
- `content`: The complete saved design JSON object, including any user revisions made during approval.
- `feedback`: The user's reason for rejection.
Args:
- `prohibitions`: Each item specifies a concrete visual restriction tailored to this presentation and shared across slides.
- `requirements`: Each item specifies a concrete visual requirement tailored to this presentation and shared across slides.

## edit_spec

**Create or update the content and layout plan of an existing slide**.
Cases:
- Establish the overall specification of an existing slide before authoring its HTML.
- Revise when the current spec conflicts with the latest manifest, design, or user requirements.
Rules:
- Omitted fields retain their saved values.
- When updating an array, provide all entries that should be present after the update, including existing entries you want to retain.
- When `purpose` is `content`, the spec must include a valid `approach` specifying the slide's content type.
Returns:
- `content`: The complete saved spec JSON object.
Args:
- `approach`: `explanation` for concepts or facts; `comparison` for differences or tradeoffs; `example` for concrete cases; `guidance` for methods or recommendations; `other` for a clearly known mode outside these categories.
- `core`: A concise statement of the main takeaway the audience should retain from this slide, rather than its title, a content inventory or layout instructions.
- `elements`: The complete ordered list of semantic content units for this slide; order follows how the audience should read them.
- `elements.intent`: A concise statement of the concrete message, facts, evidence or relationship this element should convey and why it belongs on this slide.
- `elements.type`: `text` for prose; `list` for items; `metric` for a key figure; `quote` for a quotation; `table` for tabular values; `chart` for data visualization; `diagram` for relationships; `code` for a code sample; `asset` for visual media.
- `layout`: A concise statement of how content is spatially arranged to improve visual appeal and readability, rather than specific HTML or CSS implementation details.
- `purpose`: `cover` for topic and identity; `introduction` for background, problem, goals or structure; `transition` for a chapter change; `content` for body material; `conclusion` for summary, conclusions or actions; `other` for a clearly known role outside these categories.
- `slide_id`: Stable unique slide ID from the current outline.

## edit_outline

**Create or update the presentation's sections, subsections, and slides**.
Cases:
- Establish the initial sections, subsections, and slides before authoring slide content.
- Add, remove, rename or reorganize slides, sections or subsections.
Rules:
- Supply exactly one of `content` or `edits`.
- The root requires an ordered `sections` array. Do not mix direct `slides` and `subsections` within the same section; use `[]` for unused child arrays.
- Set `id` to `"new"` for each new node. Each occurrence receives a distinct stable ID; use the IDs in the returned outline for subsequent calls.
- Preserve existing IDs when renaming, moving or reordering nodes. Do not duplicate existing IDs or change their node types.
- Any existing slide omitted from the final outline is removed along with its saved spec and HTML.
- Replacements are applied in order. Each `old_text` must match exactly once in the JSON after earlier replacements.
Returns:
- `content`: Complete saved outline JSON source as a string, including assigned stable IDs and any user revisions made during approval.
- `feedback`: The user's reason for rejection.
Example:
```json
{
  "sections": [
    {
      "id": "new",
      "title": "Introduction",
      "slides": [
        { "id": "new", "title": "Overview" }
      ],
      "subsections": []
    },
    {
      "id": "new",
      "title": "Analysis",
      "slides": [],
      "subsections": [
        {
          "id": "new",
          "title": "Key findings",
          "slides": [
            { "id": "new", "title": "Supporting evidence" }
          ]
        }
      ]
    }
  ]
}
```
Args:
- `content`: Complete outline JSON source to create or fully replace the outline, without Markdown fences.
- `edits`: Ordered exact text replacements in the current saved outline JSON source.
- `edits.new_text`: Literal replacement source; use an empty string to delete the matched text.
- `edits.old_text`: Exact, non-empty source text to replace, including whitespace and enough surrounding context to identify one occurrence.

## edit_html

**Create or update the HTML of an existing slide**.
Cases:
- Generate a slide's HTML based on the latest manifest, design, spec, and user requirements.
- Revise an existing slide's HTML to address changed requirements or review findings.
Rules:
- Supply exactly one of `content` or `edits`.
- Replacements are applied in order. Each `old_text` must match exactly once in the HTML after earlier replacements.
Args:
- `content`: The complete HTML document to create or replace, without Markdown fences.
- `edits`: The ordered list of exact text replacements to apply to the current saved HTML.
- `edits.new_text`: The literal replacement source; use an empty string to delete the matched text.
- `edits.old_text`: The exact, non-empty source text to replace, including whitespace.
- `slide_id`: Stable unique slide ID from the current outline.

## create_plan

**Create or replace an unapproved plan draft and submit it for user approval**.
Cases:
- The task is complex enough to need a plan that clarifies the steps and dependencies before execution.
- An unapproved proposal needs revision based on user feedback or clarified requirements.
Rules:
- If the user requests changes, resubmit the complete revised plan and all steps.
- `create_plan` is not limited to `plan` mode. In `execute` mode, assess the task's complexity and use it when planning is needed.
Returns:
- `feedback`: The user's reason for rejection or requested revisions.
Errors:
- `PLAN_INVALID`: The proposal or progress update violates the current plan state or locked structure.
Args:
- `content`: Complete Markdown plan organized into understanding of the request, implementation approach, detailed steps, relevant considerations, and expected outcomes. Keep the detailed steps consistent with `steps`.
- `steps`: Ordered array of strings for the complete proposed plan. Each string is a single concise sentence that precisely summarizes the step's action or deliverable recognizable in plan progress.
- `title`: User-facing title of the proposed work.

## update_plan

**Update execution statuses of existing steps in the approved active plan**.
Cases:
- A plan step's required work is complete and its expected result has been achieved.
- The recorded step statuses do not reflect the task's actual progress.
Rules:
- Check the recorded step statuses in `current_plan`.
- Omitted steps retain their current status. Do not resubmit steps already marked `completed`.
- When the work for multiple steps is complete, mark them as `completed` in a single call.
- At most one step may be `processing` at a time. Do not change a `completed` step to any other status.
Errors:
- `PLAN_INVALID`: The proposal or progress update violates the current plan state or locked structure.
Args:
- `updates`: Status changes for existing approved steps.
- `updates.status`: `pending` for work awaiting execution; `processing` for work in progress; `completed` for finished work that achieved its expected result; `failed` for work whose execution failed.
- `updates.step_id`: Exact unique step ID from the current plan context.

## request_privilege

**Request authorization for additional slides in the current run**.
Cases:
- The current authorized slide scope is insufficient to fulfill the user's latest request.
- You have to edit or render slides outside the current authorized scope.
Rules:
- Submit this call without other tool calls and wait for its result.
- Use the updated scope in `run_state` to determine which slides are authorized.
- If the request is refused, the authorized scope remains unchanged.
Returns:
- `feedback`: The user's reason for rejection or requested revisions.
Args:
- `reason`: User-facing explanation of why the authorized slide scope needs to be expanded.
- `slide_ids`: IDs of the slides to add to the current authorized scope.

## ask_user

**Ask the user for information or decisions needed for the current task**.
Cases:
- You need the user to clarify essential information or make a decision that materially affects the task's direction or outcome.
- You need the user to make presentation-wide decisions that materially affect slide creation.
Rules:
- Submit this call without other tool calls and wait for its result.
- When providing options, include only as many meaningful alternatives as needed, up to three.
- Questions in the same call must be answerable independently. Ask dependent follow-up questions only after receiving the answers they require.
- If a question is skipped, use the available information to decide how to proceed. Do not treat the skip as an explicit choice or approval.
Returns:
- `answers`: An array of strings in the same order as the submitted questions. Each string is the selected option's label, the user's verbatim text, or a message indicating that the question was skipped.
Args:
- `questions`: Ordered questions to present together, each addressing one fact or decision.
- `questions.options`: One to three preset answers for a single-choice question; custom text is always available. Omit this field or pass `[]` to show a text input instead.
- `questions.options.description`: Concrete meaning, effect or tradeoff of this option.
- `questions.options.label`: Concise and distinct option text. If you strongly recommend this option, append `(Recommended)` to its end.
- `questions.question`: Complete user-visible interrogative sentence, ending with question mark.
- `questions.reason`: Brief user-visible explanation of why the answer is needed and what subsequent work it affects.

## review_ppt

**Request an independent assessment of the current presentation and cumulative changes in this run**.
Cases:
- You have completed all creation and revision work and need an independent reviewer to conduct a comprehensive review of the entire presentation before delivery.
- You want an independent reviewer's feedback to identify areas for improvement and refine the presentation.
Rules:
- Submit this call without other tool calls and wait for its result.
- Reserve this tool for substantial changes or complex user requirements. Skip it for simple edits unless the user explicitly requests a review.
Returns:
- `decision`: `approve` for sufficient evidence that the reviewed scope meets the requirements; `revise` for important uncertainty requiring further verification; `refuse` for confirmed defects that prevent delivery.
- `reasons`: A non-empty list of findings supporting the assessment, including evidence for approval, uncertainties requiring verification, or confirmed defects affecting delivery.
Errors:
- `REVIEW_FAILED`: Required review evidence is unavailable or unusable, or the review output is invalid.
Args:
- `demand`: Self-contained instructions for the reviewer, including the review objective, key areas to focus on, acceptance criteria, relevant user requirements and decisions, and the feedback you need.

## suggest_request

**Submit useful next requests for the user to choose from**.
Rules:
- Available only in the independent suggest command.
- Submit this call exactly once, without other tools or ordinary assistant text.
- Provide zero to three distinct, ready-to-send requests in the user's language, ordered by usefulness.
- Return an empty array for greetings, acknowledgments, identity questions or when no useful continuation is supported.
- This submits suggestions only; it does not execute a request or complete an authoring task.
Args:
- `requests`: Required array of zero to three single-line plain-text requests, each 1–80 characters. Infer them from the conversation, current project and completed work, matching the user's language and wording.

## git_commit

**Submit a commit message for the staged project changes**.
Args:
- `items`: Distinct body entries, each summarizing one coherent change in a single sentence.
- `title`: Concise single-line plain-text title that summarizes the primary change, in the format `<feat|fix|refactor|perf|chore|docs>: <primary change>`, without Markdown, HTML, surrounding quotation marks, status prefixes, or a trailing period.

## run_command

**Run shell commands to inspect files or make authorized project changes**.
Cases:
- You can achieve the same result more efficiently with shell commands than with other available tools.
Rules:
- Use project-relative paths for project changes and must not write elsewhere.
- Modify only project content files and must not alter internals or perform destructive system operations.
- Refusal, failure, cancellation, or timeout prevents local changes from being saved.
Returns:
- `exit_code`: The shell's exit status.
- `stderr`: Captured standard error.
- `stdout`: Captured standard output.
- `feedback`: The user's reason for rejecting the request.
Errors:
- `COMMAND_INVALID`: The command is empty or has invalid shell syntax.
- `COMMAND_NOT_ALLOWED`: The command attempts a prohibited write or operation.
- `COMMAND_NOT_AVAILABLE`: The executable or required shell sandbox is unavailable.
- `COMMAND_EXIT_NONZERO`: The shell exited unsuccessfully.
- `COMMAND_TIMEOUT`: The command exceeded its execution deadline.
Args:
- `command`: Bash source to execute from the project directory.

## rename_thread

**Submit a decision to rename the conversation or keep its current title**.
Args:
- `action`: `rename` for a missing title or one that no longer represents the recent main topic; `keep` for a title that still fits or when the available information is insufficient to choose a better one.
- `title`: Concise single-line plain-text title that identifies the conversation's main topic or current stage of work, without Markdown, HTML, surrounding quotation marks, status prefixes, or a trailing period.

## compact_context

**Submit a concise working summary that preserves the context needed to continue the same task**.
Args:
- `content`: Complete Markdown working summary for continuing the same task.
- `title`: Concise single-line plain-text title that identifies the task, stage, or key decision captured in the summary, without Markdown, HTML, surrounding quotation marks, status prefixes, or a trailing period.

## polish_instruction

**Submit an improved version of the user's draft while preserving its intended meaning**.
Args:
- `content`: Complete revised draft ready to send, or the original draft if no changes are needed.
- `title`: Concise single-line plain-text title that describes the wording improvement or indicates that the draft is unchanged, without Markdown, HTML, surrounding quotation marks, status prefixes, or a trailing period.

## handoff_thread

**Submit a self-contained handoff brief for another conversation to continue the user's work**.
Args:
- `content`: Complete standalone Markdown handoff brief, ready to send as the first message to the receiving Agent.
- `title`: Concise single-line plain-text title that identifies the work or stage being handed over, without Markdown, HTML, surrounding quotation marks, status prefixes, or a trailing period.
