# Outline and Design Contract Design

## Scope

This design finalizes the next contract for `outline.json` and `design.json`.
`slides/<id>/spec.json` content redesign is intentionally out of scope, except
for compatibility adjustments required by resource references and validation.

## Decisions

### Runtime-Owned Fields

All durable identity and bookkeeping fields are Runtime-owned:

- `version`
- `revision`
- `project`
- `slide_id`
- `section`
- `subsection`
- source revisions
- timestamps

The model-facing contract must not ask the agent to invent durable IDs. The
agent may express ordering, section placement, titles and intent. Runtime maps
those choices to persisted IDs.

### ID Format

Durable IDs use short Runtime-generated identifiers:

| Field | Format | Uniqueness |
|---|---|---|
| `project` | `pro_<rand6>` | global |
| `slide_id` | `sli_<rand6>` | project-local |
| `section` | `sec_<rand6>` | outline-local |
| `subsection` | `sub_<rand6>` | outline-local |

The random segment uses an unambiguous lowercase base32 alphabet. Generation
must check the relevant scope and retry on collision.

### outline.json

`outline.json` owns deck goal, audience, language, optional positioning,
constraints, section structure and stable slide order.

Required model-visible content fields:

- `title`
- `goal`
- `audience`
- `language`
- `sections`
- `outline_order`

Optional model-visible content fields:

- `positioning`
- `constraints`

Removed fields:

- `schema_version`, replaced by Runtime-owned `version`
- `project_id`, replaced by Runtime-owned `project`
- `core_thesis`, replaced by optional `positioning`
- `narrative_arc`
- `sections[].number`
- `sections[].subsections[].number`

Section items contain:

- Runtime-owned `id`
- model-visible `title`
- model-visible `purpose`
- `subsections`

Subsection items contain:

- Runtime-owned `id`
- model-visible `title`

`constraints` records explicit deck-level boundaries: what must be included,
what must be avoided, style limits and content limits. It is not a substitute
for `goal`; it is the guardrail for generation.

### design.json

`design.json` does not duplicate theme tokens. The style repository theme is
the source of reusable visual system values such as palette, typography,
spacing, radius and shadows.

`design.json` owns only:

- which repository theme is selected
- the visual direction for this specific deck
- the target information density
- shared deck chrome

Target fields:

- Runtime-owned `version`
- Runtime-owned `revision`
- Runtime-owned `project`
- `theme`
- `direction`
- `density`
- `chrome`
- Runtime-owned `created_at`
- Runtime-owned `updated_at`

Removed fields:

- `schema_version`, replaced by `version`
- `project_id`, replaced by `project`
- `canvas`
- `palette`
- `typography`
- `spacing`
- `radius`
- `shadows`
- `layout_system`
- `signature`
- `motion`

`theme` is a repository theme ID. Seed themes are just initial repository
values; the schema does not distinguish seed and repo sources.

`direction` is the deck-specific visual direction on top of the selected theme,
for example: "cloud-native console, not cyberpunk poster".

`density` is one of:

- `sparse`
- `medium`
- `dense`

`chrome[]` items contain exactly:

- `type`: `page_number | section_marker | key_message | deck_title`
- `placement`: `top-left | top-center | top-right | bottom-left | bottom-center | bottom-right | left-edge | right-edge`
- `style`: short semantic style direction, not raw CSS

No `visibility` field is included. Default visibility is determined by the
renderer and component type.

### Theme Tokens

`common/tokens.css` should be produced from the selected repository theme.
`design.json` no longer rebuilds token values from palette/typography fields.

The HTML output contract remains unchanged:

- fixed 16:9 stage
- shared `common/base.css`
- shared `common/tokens.css`
- no hardcoded theme values in slide HTML

## Migration

Existing projects are migrated by normalizing resource files when read or
written by the runtime:

- `schema_version` -> `version`
- `project_id` -> `project`
- `core_thesis` -> `positioning`
- drop `narrative_arc`
- drop section/subsection `number`
- legacy design fields collapse into the new theme/direction/density/chrome
  shape

Where legacy IDs exist, a migration step must preserve reference integrity.
New resources use short Runtime-generated IDs.

## Validation

Validation must cover:

- schema structure
- unique `outline_order`
- section and subsection ID uniqueness
- slide spec references to valid sections/subsections
- theme ID format and token availability
- chrome enum values

## Implementation Notes

- Keep database column names such as `project_id` unchanged for runtime
  storage. The rename to `project` applies to PPT JSON resources.
- Public SSE `schema_version: 2` is a separate event protocol and is not part
  of this resource schema change.
- `spec.json` semantic redesign is deferred.
