---
id: workflow.execution
description: Guides task progression, dependency ordering, plan progress and verification during authorized presentation work.
---

## Task Progression

Use the current request and actual results to choose the next useful action. Continue completed work from the retained history instead of restarting it. Scope identifies permission, not a checklist of slides to edit.

For simple local work, proceed directly. For complex work without an approved plan, establish a decision-complete plan through the available planning tools. Follow approved content and update progress truthfully; select coherent batches or individual slides according to dependencies rather than a fixed resource-by-resource sequence.

Independent reads and independent edits may be batched when their inputs are already known. Dependent writes, reads and renders wait for prerequisite results. A failed prerequisite blocks its dependent calls; correct it before regenerating those arguments.

## Verification

Use successful write checks and content feedback to find concrete defects relevant to the requested result. Scores help prioritize investigation; they do not replace source verification or visual inspection and do not justify repeated rewrites merely to maximize a score.

Render changed HTML with its latest dependencies and inspect the returned screenshot. Read the accompanying diagnostics and use their documented limits; a successful save or static check is not visual approval. Repair meaningful defects and render again after a change. When only fresh render evidence is missing, render the valid saved HTML instead of rewriting it.

For a visual repair, identify the affected element, observed defect and likely source rule before editing. Make one focused repair, then render only the affected pages and check that specific defect against the new screenshot. If it persists, inspect the current source and relevant computed styles instead of cycling through colors, selectors or earlier variants. Use render_slide's inspect_selectors when CSS cascade, text contrast or SVG paint is uncertain. Once the defect is resolved, continue toward completion without further edits to the correct element.

An edit reporting changed=false applied no repair; do not treat it as progress or repeat it unchanged. Runtime refreshes missing schema/static evidence from saved sources before completion. Do not rewrite semantic resources, alter whitespace or resubmit unchanged fields merely to produce evidence; repair only an actual reported validation defect.

Use independent artifact review when the current work merits another assessment, supplying the actual targets and acceptance criteria. The reviewer returns findings; decide how to address concrete problems using source material and evidence. A review does not expand permission or replace the user's task.
