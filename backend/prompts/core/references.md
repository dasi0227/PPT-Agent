---
id: core_references
desc: Specify the significances of the references.
---

## Project Files

| Resource      | Meaning                                                                | Purpose                                                                                     |
| ------------- | ---------------------------------------------------------------------- | ------------------------------------------------------------------------------------------- |
| `meta`        | Presentation basics.                                                   | Maintain the presentation’s basic information.                                              |
| `outline`     | Ordered sections, subsections, and slides with stable IDs.             | Locate slides and determine their current pagination, titles, hierarchy, and order.         |
| `constraints` | Presentation-wide content and visual requirements and restrictions.    | Guide content selection and visual design across slides.                                    |
| `spec`        | The intended message, content, and composition of an individual slide. | Guide authoring and layout organization when creating or modifying the corresponding slide. |
| `html`        | The authored implementation of an individual slide.                    | Read, edit and render the slide's source code to inspect if it meets the requirements.      |

## Runtime Context

| Field                  | Meaning                                                                              | Purpose                                                                                                                                               |
| ---------------------- | ------------------------------------------------------------------------------------ | ----------------------------------------------------------------------------------------------------------------------------------------------------- |
| `run_state`            | Current mode, authorized slide IDs, and execution phase.                             | Determine permitted actions, check scope before editing, and continue from the current phase.                                                         |
| `project_state`        | Resource availability, slide render states, and cumulative changes.                  | Determine what to read, create, render, or verify. Identify missing outputs, check screenshot freshness, and locate changed resources for inspection. |
| `current_plan`         | Current execution steps and their progress.                                     | Continue applicable steps and keep progress aligned with actual work.                                           |
| `available_skills`     | Catalog of available skills, with identities and descriptive metadata.               | Use this catalog to answer questions about currently available skills by name and description. Load a skill when the task needs its methods.                                                                          |
| `active_skills`        | Complete instructions for currently loaded skills.                                   | Apply relevant methods, workflows, and quality criteria to the task.                                                                                  |
| `available_components` | Catalog of available component references, with identities and descriptive metadata. | Select and load components suited to the slide's content and composition.                                                                             |
| `active_components`    | Complete HTML source for currently loaded components.                                | Inspect and adapt their structure, styling, and behavior to the slide's content and composition.                                                      |

## User's Material 

| Resource     | Meaning                                                                                     | Purpose                                                                                         |
| ------------ | ------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------- |
| `attachment` | Uploaded images with attachment IDs and original asset paths.                               | Inspect relevant material, extract needed content, and incorporate suitable assets into slides. |
| `annotation` | Comments attached to selected elements or regions, with location and captured page content. | Interpret the user's comment, locate the intended target, and guide exact edits.                |
