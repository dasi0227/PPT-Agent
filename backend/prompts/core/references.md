---
id: core_references
desc: Specify the significances of the references.
---

## Project Files

- `meta`: Presentation basics, used to define the title, language, slide count, audience, and goal.
- `outline`: Ordered sections, subsections, and slides with stable IDs, used to determine the hierarchy and pagination.
- `constraints`: Presentation-wide content and visual requirements and restrictions, used to guide content selection and slide design.
- `spec`: An individual slide’s intended message, content, and composition, used to guide its creation and revision.
- `html`: Slide HTML source, used to edit, render, and inspect the slide.

## Runtime Context

- `run_state`: Current mode, authorized slide IDs, and execution phase, used to determine permitted actions and edit scope.
- `project_state`: Resource availability, render states, and cumulative changes, used to identify missing outputs, outdated screenshots, and resources requiring inspection.
- `current_plan`: Execution steps and their progress, used to resume unfinished work and track completion.
- `available_skills`: Skill names and descriptions, used to select suited skills to load.
- `active_skills`: Loaded skill instructions, used to apply task-specific methods, workflows, and quality criteria.
- `available_components`: Component names and descriptions, used to select suited components to load.
- `active_components`: Loaded component HTML, used to adapt structure, styling, and behavior to the slide.

## User's Material 

- `attachment`: Uploaded images with attachment IDs and original asset paths, used to extract content and incorporate assets into slides.
- `annotation`: Comments on selected elements or regions with location and captured page content, used to locate edit targets and interpret requested changes.
