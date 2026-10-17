---
id: core.reference
description: Define the content and significance of the references.
---

## Project File

| Resource   | Meaning                                                                    | Purpose                                                                                            |
| ---------- | -------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------- |
| `outline`  | Ordered sections, subsections, and slides with stable IDs.                 | Locate slides and determine their current pagination. Manage titles, hierarchy, and order.         |
| `manifest` | Presentation-wide goals, audience, content requirements, and restrictions. | Guide narrative planning and content selection. Determine content scope and intended outcome.      |
| `design`   | Presentation-wide visual requirements and prohibitions.                    | Guide overall consistent visual decisions and frontend style when creating or modifying slides.    |
| `spec`     | The intended message, content, and composition of an individual slide.     | Guide slide authoring and layout direction. Determine what should be conveyed and how to organize. |
| `html`     | The authored implementation of an individual slide.                        | Read and edit the slide's source code. Render to inspect if it meets the requirements.             |

## Runtime Context

| Field                  | Meaning                                                                              | Purpose                                                                                                                                               |
| ---------------------- | ------------------------------------------------------------------------------------ | ----------------------------------------------------------------------------------------------------------------------------------------------------- |
| `run_state`            | Current mode, authorized slide IDs, and execution phase.                             | Determine permitted actions, check scope before editing, and continue from the current phase.                                                         |
| `project_state`        | Resource availability, slide render states, and cumulative changes.                  | Determine what to read, create, render, or verify. Identify missing outputs, check screenshot freshness, and locate changed resources for inspection. |
| `current_plan`         | Current plan, step progress, and approval state.                                     | Follow the applicable plan only when approved is true. Continue and update progress to reflect actual work.                                           |
| `available_skills`     | Catalog of available skills, with identities and descriptive metadata.               | Select and load skills that provide specialized methods needed for the task.                                                                          |
| `active_skills`        | Complete instructions for currently loaded skills.                                   | Apply relevant methods, workflows, and quality criteria to the task.                                                                                  |
| `available_components` | Catalog of available component references, with identities and descriptive metadata. | Select and load components suited to the slide's content and composition.                                                                             |
| `active_components`    | Complete HTML source for currently loaded components.                                | Inspect and adapt their structure, styling, and behavior to the slide's content and composition.                                                      |

## User's Material 

| Resource     | Meaning                                                                                     | Purpose                                                                                         |
| ------------ | ------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------- |
| `attachment` | Uploaded images with attachment IDs and original asset paths.                               | Inspect relevant material, extract needed content, and incorporate suitable assets into slides. |
| `annotation` | Comments attached to selected elements or regions, with location and captured page content. | Interpret the user's comment, locate the intended target, and guide exact edits.                |
