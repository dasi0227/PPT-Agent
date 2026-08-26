# Slide Spec and Materialization Contract Design

## Scope

This design finalizes `slides/<slide_id>/spec.json` and moves slide HTML
materialization metadata from SQLite into a Runtime-owned file:

```text
slides/<slide_id>/materialization.json
```

The existing `outline.json` and `design.json` contracts remain unchanged.

## Ownership

### Authoring Resources

- `outline.json` owns deck intent, constraints, section structure and slide order.
- `design.json` owns theme selection, deck direction, density and shared chrome.
- `slides/<slide_id>/spec.json` owns one page's semantic plan.
- `slides/<slide_id>/index.html` is the final page implementation.

### Runtime Resource

`materialization.json` records the last successfully rendered relationship
between one HTML artifact and the exact outline, spec and design revisions used
to render it. The complete file is Runtime-owned and is never exposed as an
Agent-writable resource.

## spec.json

### Target Shape

```json
{
  "version": "3.0",
  "revision": 4,
  "project": "pro_k7m2qx",
  "slide_id": "sli_8n4wcp",
  "section": "sec_a7m2kx",
  "subsection": "sub_q9n3wd",
  "role": "evidence",
  "title": "企业客户成为增长主引擎",
  "key_message": "企业客户贡献了超过七成的新增收入",
  "elements": [
    {
      "type": "metric",
      "intent": "突出企业收入同比增长 42%"
    },
    {
      "type": "chart",
      "intent": "展示近三年企业收入增长趋势"
    },
    {
      "type": "asset",
      "intent": "使用企业团队分析数据的写实辅助画面"
    }
  ],
  "layout": "two-column",
  "created_at": 1786200000,
  "updated_at": 1786201200
}
```

### Runtime-Owned Fields

- `version`
- `revision`
- `project`
- `slide_id`
- `section`
- `subsection`
- `created_at`
- `updated_at`

The persisted JSON contains section references, but the Agent contract strips
them. Normal Spec writes preserve the current placement. Section moves use the
explicit restructure operation, where Runtime resolves placement to durable
IDs.

### Agent-Writable Fields

- `role`: required semantic string, not a fixed enum.
- `title`: required page title.
- `key_message`: required single-page thesis.
- `elements`: required ordered array of semantic planning elements.
- `layout`: optional non-empty string. This change does not introduce a static
  enum or repository existence validation.

Each `elements[]` item contains exactly:

- `type`: `text | list | metric | quote | table | chart | diagram | code | asset`
- `intent`: concrete natural-language planning instruction.

Elements do not contain IDs, coordinates, CSS, `items`, `data`, payload ASTs or
component implementation details. `type: "asset"` replaces the old
`visual_intent.asset_queries` and separate asset-intent proposals.

### Removed Fields

- `schema_version`, replaced by `version`
- `project_id`, replaced by `project`
- `source_outline_revision`
- `content`
- `visual_intent`
- `speaker_notes`
- proposed `composition`
- proposed `asset_intents`

### key_message and Chrome

`spec.key_message` is the only text source for the page thesis.
`design.chrome[type=key_message]` only supplies shared placement and semantic
style. It never stores another message value. When that chrome item exists, the
HTML implementation renders `spec.key_message` in the shared slot. Without it,
the message remains a semantic planning and review input without requiring a
dedicated chrome element.

## materialization.json

### Target Shape

```json
{
  "version": "3.0",
  "artifact": {
    "revision": 3,
    "hash": "sha256:4a75..."
  },
  "source": {
    "outline": 5,
    "spec": 7,
    "design": 2,
    "hash": "sha256:91bc..."
  },
  "rendered_at": 1786201200
}
```

### Semantics

- `version` is the materialization schema version.
- `artifact.revision` increments only when `index.html` content changes.
- `artifact.hash` is the SHA-256 hash of the exact HTML bytes.
- `source.outline`, `source.spec` and `source.design` are the exact current
  resource revisions used by the successful render.
- `source.hash` is a deterministic SHA-256 hash over the exact outline, spec
  and design bytes in a fixed order.
- `rendered_at` is the successful render timestamp.

The materialization file has no independent content revision. Runtime replaces
it atomically after successful render evidence. A design-only rerender may
update `source.design` while leaving `artifact.revision` unchanged.

### State Derivation

```text
HTML missing
  -> not_materialized

HTML exists but materialization is missing, invalid, or artifact hash differs
  -> unknown

source.outline < outline.revision OR source.spec < spec.revision
  -> spec_stale

source.design < design.revision
  -> design_stale

all revisions equal and hashes match
  -> fresh
```

Revision values greater than the current resource revision or mismatched source
hashes are invalid and produce `unknown`; they are not treated as fresh.

## SQLite Boundary

Remove current-resource revision mirrors:

- `projects.outline_revision`
- `projects.design_revision`
- `slides.spec_revision`

Move HTML materialization metadata out of SQLite:

- `slides.html_revision`
- `slides.source_outline_revision`
- `slides.source_spec_revision`
- `slides.source_design_revision`

The `slides` table retains:

- `id`
- `project_id`
- `current_version`
- `last_export_at`

`current_version` remains because it is the rollback/version-snapshot pointer,
not a content revision mirror.

Project and slide API responses may retain their existing revision fields for
compatibility, but services derive them from JSON and materialization files.

## Write and Commit Flow

1. Runtime normalizes and writes outline, design, spec and HTML resources.
2. Chromium renders the exact HTML against the current project files.
3. Render evidence records artifact hash, source hash and all revisions.
4. Commit verifies the evidence against the latest exact file bytes.
5. Runtime writes version snapshots.
6. Runtime atomically replaces `materialization.json` last.
7. SQLite commits identity, rollback pointer, version rows and run metadata.

If the final SQLite commit fails, Runtime restores the previous
`materialization.json`. If materialization writing fails, the commit fails and
the old or absent file keeps the page stale or unknown.

Rollback replaces HTML from a snapshot and removes its materialization file.
The restored HTML must be rendered again before it can become fresh.

## Migration

The SQLite migration copies legacy slide materialization columns into a
temporary backup table before rebuilding `projects` and `slides` without
revision mirrors.

At startup, the filesystem migration:

1. normalizes every current slide spec to the new contract;
2. normalizes slide-spec snapshots;
3. reads legacy materialization backup rows;
4. writes `materialization.json` only when HTML exists and all legacy source
   revisions are valid;
5. validates generated files;
6. removes the temporary backup table only after all projects succeed.

Missing or incomplete legacy materialization data is not fabricated. Such
slides become `unknown` until rendered again.

## Error Handling

- Missing or invalid authoring JSON remains a project read error.
- Missing HTML produces `not_materialized`.
- Invalid materialization JSON or hash mismatch produces `unknown`.
- Stale render evidence is rejected before commit.
- Materialization is never Agent-writable through `mutate_ppt` or `mutate_ppt`.

## Validation

Coverage must include:

- positive and negative JSON Schema cases for both resources;
- Agent contract exclusion of all Runtime-owned spec fields;
- migration from legacy spec and DB materialization columns;
- file revision conflict behavior;
- state derivation from materialization files;
- render-proof rejection for stale artifact/source hashes;
- design-only rerender without HTML revision increment;
- rollback invalidating materialization;
- API revision projection from files;
- frontend rendering of `elements`;
- Go package tests, TypeScript checks, frontend tests and production build.
