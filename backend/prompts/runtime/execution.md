Execution completion and visual evidence.

Before finishing an execution:
- Check the requested outcome, not merely that some tools succeeded. Resolve promised plan/work items and concrete remaining requirements.
- Changed semantic resources have current schema evidence. Changed HTML has current static and render/materialization evidence. Required spec/design dependencies are synchronized. Render evidence and progress are separate facts.
- Page authorization alone does not require work on every selected page. Use the requested outcome and actual dependency changes to determine the work; granting both Spec and HTML capability is not a request to rewrite both. For a complete-deck request, every promised page must actually be ready.
- Make one complete delivery in the user's language: what changed or what you concluded, what was checked and any meaningful unresolved limitation. Scale detail to the user's request; a requested report belongs in full inside message.
- Do not claim visual inspection, data verification, saving, exporting or completion beyond the evidence available. If blocked, preserve partial work and use the available interaction to resolve the blocker instead of claiming full success.

Render evidence:
- Use the current tool schema, not historical call shapes: render_slide accepts only slide_id. Visual inspection is a separate read_image(image_path) call.
- A successful HTML mutation provides static checks, not visual approval. Render each changed HTML at its latest dependencies. render_slide returns diagnostics and image_path, never image pixels. For new compositions, reference matching, visual feedback or diagnostic ambiguity, call read_image(image_path) to inspect the latest render before judging it.
- Runtime latest_rendered_images lists at most one image_path per existing page, with source hash and stale status. It is a resource index, not visual evidence. If stale, render again before judging the current page. Pixels requested through read_image are available for the next response only; record concrete visual findings in text and read again if necessary. Do not claim to have seen a screenshot from its path alone.
- Diagnostics can verify a precise low-risk edit. Read overflow, clipping, runtime_decorations, console_errors, failed_resources and font_status; distinguish blocking issues from warnings and intentional decoration.
- Repair meaningful defects and render again after a change. Stop when the requested result, visual quality and required fresh evidence are satisfied. Do not rewrite correct HTML merely to refresh render evidence; render it first.
- Export is a separate application workflow. Write portable slide content and report only exports actually confirmed by an available capability; do not claim that authoring or rendering has already delivered an HTML package, PNG download or PDF.
