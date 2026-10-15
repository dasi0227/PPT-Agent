package tooldefinition

type Schema struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

// Get constructs a fresh definition.
func Get(id string) Schema {
	description := ""
	if id == "load_component" {
		description = "**Load component HTML sources as references for constructing presentation pages**.\n" +
			"Cases:\n" +
			"- You need to reuse an existing HTML component or read it as a reference.\n" +
			"Rules:\n" +
			"- A successful call adds the complete source to `active_components` for the current run and returns only `status=success`.\n" +
			"- Do not load a component again if it is already present in `active_components`."
	} else if id == "load_skill" {
		description = "**Load skill instructions to guide presentation creation**.\n" +
			"Cases:\n" +
			"- You need to consult an existing skill's workflows, methods, or guidelines to improve the quality of your work.\n" +
			"Rules:\n" +
			"- A successful call adds the complete skill instructions to `active_skills` for the current run and returns only `status=success`.\n" +
			"- Do not load a skill again if it is already present in `active_skills`."
	} else if id == "read_resource" {
		description = "**Read the complete current saved content of a presentation resource**.\n" +
			"Cases:\n" +
			"- You need the latest saved content to analyze the presentation or prepare an edit.\n" +
			"- Previously read content is stale, or an edit reports a content conflict that requires rereading the affected resource.\n" +
			"Rules:\n" +
			"- Use `project_state` to decide whether to read a resource: reuse `fresh` content, read `stale` resources when needed, and do not read resources marked `missing`.\n" +
			"- `slide_id` is required when `type` is `spec` or `html` and must be omitted for all other resource types.\n" +
			"Returns:\n" +
			"- `content`: Complete current saved source as a string: original JSON source for manifest, design, outline and spec; original HTML source for html."
	} else if id == "read_image" {
		description = "**View an uploaded image or the latest screenshot of a slide**.\n" +
			"Cases:\n" +
			"- Inspect an uploaded image to understand what the user is referring to and what they want to achieve.\n" +
			"- Inspect a slide screenshot to check whether the HTML renders as intended.\n" +
			"Rules:\n" +
			"- Supply exactly one of `attachment_id` or `slide_id`. Do not provide both in the same call.\n" +
			"- A successful call returns the requested image directly for visual inspection.\n" +
			"Errors:\n" +
			"- `IMAGE_INVALID`: The image exists but is empty, damaged or unsupported."
	} else if id == "render_slide" {
		description = "**Render the saved slide HTML into a screenshot, report rendering problems, and optionally assess page quality**.\n" +
			"Cases:\n" +
			"- Inspect how a newly created or modified slide actually renders and whether it meets the user's requirements.\n" +
			"- Investigate a reported layout or rendering problem, or verify that a fix worked.\n" +
			"- You need additional feedback on a slide's content coverage, expression clarity, or adherence to requirements.\n" +
			"Rules:\n" +
			"- If the slide's `render` state in `project_state` is `fresh` and you only need to view the screenshot, reuse it from context or use `read_image`.\n" +
			"- Set `need_preview` to `true` when an additional quality assessment would help and is really necessary.\n" +
			"- Successful rendering alone does not establish that the slide meets the requirements.\n" +
			"- Treat `preview` as advisory feedback on the current slide. Verify indicated problems against the actual page and current requirements before making changes.\n" +
			"Returns:\n" +
			"- `diagnostics`: Observed rendering problems.\n" +
			"- `diagnostics.overflow`: Horizontal and vertical overflow of scrollable content beyond the canvas viewport, with observed dimensions and excess pixels.\n" +
			"- `diagnostics.console_errors`: Browser console errors or uncaught script errors.\n" +
			"- `diagnostics.failed_resources`: Deduplicated failed requests, HTTP 400+ responses or blocked external resources.\n" +
			"- `diagnostics.out_of_bounds`: Descendants extending beyond .slide-stage (document root if absent). Entries contain `tag`, optional `id`/`class`, and `rect` with `left`/`top`/`right`/`bottom` edges in viewport CSS pixels rounded to integers.\n" +
			"- `preview`: Optional present assessment.\n" +
			"- `preview.scores`: Three independent scores from 0 to 3, higher is better; fractional values are probability-weighted means of rubric levels, not exact defect findings or an overall pass score.\n" +
			"- `preview.scores.content_coverage`: Coverage of this page’s intended message and required support. `0`: absent or unrelated; `1`: central message or major material missing; `2`: central message and most material present with specific omissions; `3`: fully performs this page’s intended role.\n" +
			"- `preview.scores.expression_clarity`: Clarity of the main message, organization and wording. `0`: no understandable message or organization; `1`: meaning must be guessed; `2`: clear message with limited ambiguity or complexity; `3`: consistently clear conclusion, supporting organization and wording.\n" +
			"- `preview.scores.requirement_adherence`: Compliance with page-applicable requirements and restrictions in Manifest, Design and Spec. `0`: contradicts a key requirement; `1`: material deviations; `2`: core requirements met with minor deviations; `3`: fully follows applicable requirements and restrictions.\n" +
			"- `preview.judges`: Two independent single-choice judgments; one primary mismatch source and one primary page issue.\n" +
			"- `preview.judges.mismatch`: `manifest_mismatch`: violates applicable global content, audience, language, goals or content restrictions; `spec_mismatch`: misses explicit core, content, approach or layout planning; `design_mismatch`: violates explicit visual requirements or prohibitions; `none`: none of these resource mismatches was observed.\n" +
			"- `preview.judges.issue`: `expression_issue`: unclear logic, organization, wording or excessive redundancy; `layout_issue`: overlap, cropping, misalignment, spatial grouping or position-based reading order obstructs reading; `visual_issue`: problematic color, typography, contrast, emphasis, hierarchy or style beyond concrete spatial errors; `none`: none of these page issues was observed.\n" +
			"Errors:\n" +
			"- `RENDER_FAILED`: The page violates HTML authoring requirements or its observed rendering diagnostics report overflow, out-of-bounds elements, console errors or failed resources."
	} else if id == "edit_manifest" {
		description = "**Create or update presentation-wide content demands**.\n" +
			"Cases:\n" +
			"- Establish the overall content of the presentation before starting to generate the pages.\n" +
			"- The user explicitly requests changes to presentation-wide content requirements rather than requirements for an individual slide.\n" +
			"Rules:\n" +
			"- Omitted fields retain their saved values.\n" +
			"- When updating an array, provide all entries that should be present after the update, including existing entries you want to retain.\n" +
			"- `status=refuse` means the user rejected the proposed edit and no changes were saved.\n" +
			"Returns:\n" +
			"- `content`: The complete saved manifest JSON object, including any user revisions during approval.\n" +
			"- `feedback`: The user's reason for rejection."
	} else if id == "edit_design" {
		description = "**Create or update presentation-wide visual demands**.\n" +
			"Cases:\n" +
			"- Establish the overall visual direction of the presentation before starting to generate the pages.\n" +
			"- The user explicitly requests changes to presentation-wide visual requirements rather than requirements for an individual slide.\n" +
			"Rules:\n" +
			"- Omitted fields retain their saved values.\n" +
			"- When updating an array, provide all entries that should be present after the update, including existing entries you want to retain.\n" +
			"- `status=refuse` means the user rejected the proposed edit and no changes were saved.\n" +
			"- Describe visual requirements and restrictions in natural language. Do not include frontend code or instructions for using themes.\n" +
			"Returns:\n" +
			"- `content`: The complete saved design JSON object, including any user revisions made during approval.\n" +
			"- `feedback`: The user's reason for rejection."
	} else if id == "edit_spec" {
		description = "**Create or update the content and layout plan of an existing slide**.\n" +
			"Cases:\n" +
			"- Establish the overall specification of an existing slide before authoring its HTML.\n" +
			"- Revise when the current spec conflicts with the latest manifest, design, or user requirements.\n" +
			"Rules:\n" +
			"- Omitted fields retain their saved values.\n" +
			"- When updating an array, provide all entries that should be present after the update, including existing entries you want to retain.\n" +
			"- When `purpose` is `content`, the spec must include a valid `approach` specifying the page's content type.\n" +
			"Returns:\n" +
			"- `content`: The complete saved spec JSON object."
	} else if id == "edit_outline" {
		description = "**Create or update the presentation's page structure**.\n" +
			"Cases:\n" +
			"- Establish the initial sections, subsections and slides before generating pages.\n" +
			"- Add, remove, move or rename slides and reorganize chapters.\n" +
			"Rules:\n" +
			"- Supply exactly one of `content` or `edits`: `content` creates a missing outline; `edits` modifies an existing outline.\n" +
			"- Replacements are applied in order. Each `old_text` must match exactly once in the JSON after earlier replacements.\n" +
			"- The root requires an ordered `sections` array; use `[]` for an empty outline.\n" +
			"- Sections contain `id`, `title`, `purpose`, `slides` and `subsections`; subsections contain `id`, `title`, `purpose` and `slides`; slides contain only `id` and `title`.\n" +
			"- `title` is the user-visible node title; `purpose` describes what a section or subsection contributes to the narrative.\n" +
			"- Every node requires `id`: use `\"new\"` for each new node and preserve the real ID of each existing node when renaming, moving or reordering it.\n" +
			"- Multiple nodes may use `\"new\"`; Runtime assigns each a distinct real ID after all replacements and returns the saved outline with those IDs.\n" +
			"- Do not omit `id`, invent real IDs, duplicate an existing ID or change its node type.\n" +
			"- New sections and subsections may omit child arrays, which default to `[]`; existing nodes must retain required arrays, and explicit `null` is invalid.\n" +
			"- Each section uses either direct `slides` or `subsections`, with the unused array empty.\n" +
			"- Removing a slide also deletes its spec and HTML; replacing a slide's existing ID with `\"new\"` deletes the old node and creates a new one without inheriting its files.\n" +
			"- The complete edit must produce a valid outline and is saved atomically.\n" +
			"- `status=refuse` means the proposed edit was not saved.\n" +
			"Returns:\n" +
			"- `content`: Complete actually saved outline JSON source string, including assigned stable IDs and any user revisions during approval.\n" +
			"- `feedback`: The user's reason for rejection; returned only on `status=refuse` when the user provides feedback.\n" +
			"Errors:\n" +
			"- `OUTLINE_ALREADY_EXISTS`: `content` was supplied, but an outline already exists; use `edits` to modify it.\n" +
			"- `EDIT_ANCHOR_AMBIGUOUS`: An exact `old_text` anchor occurs more than once.\n" +
			"- `EDIT_ANCHOR_NOT_FOUND`: An exact `old_text` anchor does not occur in the current source."
	} else if id == "edit_html" {
		description = "**Create or update the HTML of an existing slide**.\n" +
			"Cases:\n" +
			"- Generate a slide's HTML based on the latest manifest, design, spec, and user requirements.\n" +
			"- Revise an existing slide's HTML to address changed requirements or review findings.\n" +
			"Rules:\n" +
			"- Supply exactly one of `content` or `edits`.\n" +
			"- Replacements are applied in order. Each `old_text` must match exactly once in the HTML after earlier replacements.\n" +
			"Errors:\n" +
			"- `EDIT_ANCHOR_AMBIGUOUS`: An exact `old_text` anchor occurs more than once.\n" +
			"- `EDIT_ANCHOR_NOT_FOUND`: An exact `old_text` anchor does not occur in the current source."
	} else if id == "create_plan" {
		description = "**Create or replace an unapproved plan draft and submit it for user approval**.\n" +
			"Cases:\n" +
			"- The task is complex enough to need a plan that clarifies the steps and dependencies before execution.\n" +
			"- An unapproved proposal needs revision based on user feedback or clarified requirements.\n" +
			"Rules:\n" +
			"- If the user requests changes, resubmit the complete revised plan and all steps.\n" +
			"- `create_plan` is not limited to `plan` mode. In `execute` mode, assess the task's complexity and use it when planning is needed.\n" +
			"Returns:\n" +
			"- `feedback`: The user's reason for rejection or requested revisions.\n" +
			"Errors:\n" +
			"- `PLAN_INVALID`: The proposal or progress update violates the current plan state or locked structure."
	} else if id == "update_plan" {
		description = "**Update execution statuses of existing steps in the approved active plan**.\n" +
			"Cases:\n" +
			"- A plan step's required work is complete and its expected result has been achieved.\n" +
			"- The recorded step statuses do not reflect the task's actual progress.\n" +
			"Rules:\n" +
			"- Check the recorded step statuses in `current_plan`.\n" +
			"- Omitted steps retain their current status. Do not resubmit steps already marked `completed`. \n" +
			"- When the work for multiple steps is complete, mark them as `completed` in a single call.\n" +
			"- At most one step may be `processing` at a time. Do not change a `completed` step to any other status.\n" +
			"Errors:\n" +
			"- `PLAN_INVALID`: The proposal or progress update violates the current plan state or locked structure."
	} else if id == "request_privilege" {
		description = "**Request authorization for additional slides in the current run**.\n" +
			"Cases:\n" +
			"- The current authorized slide scope is insufficient to fulfill the user's latest request.\n" +
			"- You have to edit or render slides outside the current authorized scope.\n" +
			"Rules:\n" +
			"- Submit this call without other tool calls and wait for its result.\n" +
			"- Use the updated scope in `run_state` to determine which slides are authorized.\n" +
			"- If the request is refused, the authorized scope remains unchanged.\n" +
			"Returns:\n" +
			"- `feedback`: The user's reason for rejection or requested revisions."
	} else if id == "ask_user" {
		description = "**Ask the user for information or decisions needed for the current task**.\n" +
			"Cases:\n" +
			"- You need the user to clarify essential information or make a decision that materially affects the task's direction or outcome.\n" +
			"- You need the user to make presentation-wide decisions that materially affect slide creation.\n" +
			"Rules:\n" +
			"- Submit this call without other tool calls and wait for its result.\n" +
			"- When providing options, include only as many meaningful alternatives as needed, up to three.\n" +
			"- Questions in the same call must be answerable independently. Ask dependent follow-up questions only after receiving the answers they require.\n" +
			"- If a question is skipped, use the available information to decide how to proceed. Do not treat the skip as an explicit choice or approval.\n" +
			"Returns:\n" +
			"- `answers`: An array of strings in the same order as the submitted questions. Each string is the selected option's label, the user's verbatim text, or a message indicating that the question was skipped."
	} else if id == "review_ppt" {
		description = "**Request an independent assessment of the current presentation and cumulative changes in this run**.\n" +
			"Cases:\n" +
			"- You have completed all creation and revision work and need an independent reviewer to conduct a comprehensive review of the entire presentation before delivery.\n" +
			"- You want an independent reviewer's feedback to identify areas for improvement and refine the presentation.\n" +
			"Rules:\n" +
			"- Submit this call without other tool calls and wait for its result.\n" +
			"- Reserve this tool for substantial changes or complex user requirements. Skip it for simple edits unless the user explicitly requests a review.\n" +
			"Returns:\n" +
			"- `decision`: `approve` for sufficient evidence that the reviewed scope meets the requirements; `revise` for important uncertainty requiring further verification; `refuse` for confirmed defects that prevent delivery.\n" +
			"- `reasons`: A non-empty list of findings supporting the assessment, including evidence for approval, uncertainties requiring verification, or confirmed defects affecting delivery.\n" +
			"Errors:\n" +
			"- `REVIEW_FAILED`: Required review evidence is unavailable or unusable, or the review output is invalid."
	} else if id == "finish_task" {
		description = "**Deliver the final answer and complete the current task**.\n" +
			"Cases:\n" +
			"- You are ready to deliver a complete response to the user's current question, discussion, or clarification request.\n" +
			"- You have completed the requested creation or revision work and the necessary checks, and are ready to deliver the results.\n" +
			"Rules:\n" +
			"- Submit this call without other tool calls. Ordinary assistant text does not complete the task.\n" +
			"- Put the complete final answer in `message`. A successful call delivers it to the user and ends the task without a tool reply.\n" +
			"- Omit `suggested_next_inputs` for greetings, acknowledgments, identity questions, or exchanges without a useful continuation. \n" +
			"Errors:\n" +
			"- `COMPLETION_GATE_BLOCK`: Required work or evidence still blocks completion."
	} else if id == "git_commit" {
		description = "**Submit a commit message for the staged project changes**."
	} else if id == "rename_thread" {
		description = "**Submit a decision to rename the conversation or keep its current title**."
	} else if id == "compact_context" {
		description = "**Submit a concise working summary that preserves the context needed to continue the same task**."
	} else if id == "polish_instruction" {
		description = "**Submit an improved version of the user's draft while preserving its intended meaning**."
	} else if id == "handoff_thread" {
		description = "**Submit a self-contained handoff brief for another conversation to continue the user's work**."
	}
	return Schema{Name: id, Description: description, Parameters: inputSchema(id)}
}
