Task playbook: empty deck generation.

Build a complete presentation with a clear narrative, page-specific visual composition and usable HTML. Use the current user goal, audience, language and page range; consult the manifest only for facts not already available or needing change.

Real dependencies:
- Creating structure and changing deck-wide resources require global scope. Request the necessary expansion before attempting them.
- When the outline has no sections, outline.init accepts the proposed section/subsection/page tree with client_ref values. For existing empty sections, use disclosed outline.insert operations instead. Never reinitialize an existing structure.
- Observe the returned client_ref-to-ID mapping and canonical revisions before writing pages. Runtime creates all formal IDs.
- Establish or reuse the global design direction before authoring dependent HTML. Do not write design merely to restate an unchanged direction.
- For each new page, its outline identity and semantic spec must exist before rendering its HTML. Every declared page needs a valid spec by completion; a complete-presentation request also needs HTML and fresh render evidence for every promised page.

Choose the schedule:
- You may finish spec → HTML → render for one page, then continue; or work in coherent batches. Outline display order does not force authoring order.
- An early representative page can test typography, density and the chosen visual language before repeating it. Use different compositions for different messages while retaining the deck's visual system.
- Repair concrete defects on the affected pages, then render their latest versions. Do not defer every visual decision until the whole deck is written.
- Preserve unfinished obligations in an execution checklist when useful, and finish only after the requested deck is complete.
