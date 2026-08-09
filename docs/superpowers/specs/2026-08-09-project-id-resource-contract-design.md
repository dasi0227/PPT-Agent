# Project ID Resource Contract Design

## Decision

All PPT authoring resource schemas use `project_id` for the Runtime-owned
project identity field:

- `outline.json`
- `design.json`
- `slides/<slide_id>/spec.json`

This decision supersedes the earlier `project` field naming in the outline,
design and slide-spec contract documents. It does not change the ID value,
ownership, generation strategy or visibility. The value remains the
Runtime-generated project ID and is hidden from Agent-writable contracts.

`materialization.json` has no project identity field and is unchanged.

## Runtime Contract

The Go domain field remains `ProjectID`; only its JSON tag changes from
`project` to `project_id`. TypeScript resource types expose the same
`project_id` property.

Runtime normalization always writes `project_id` and treats both `project` and
`project_id` as controlled input paths. Agent contracts strip `project_id`
because the schemas mark it with `x-runtime-managed`.

No resource may persist both fields. JSON Schema continues to reject the
obsolete `project` field through `additionalProperties: false`.

## Migration

The project filesystem layout version advances from 3 to 4.

At startup, layout migration normalizes current outline, design and slide-spec
files plus their version snapshots:

1. Set `project_id` to the authoritative database project ID.
2. Delete the obsolete `project` field.
3. Preserve all other resource data and revisions.
4. Recompute a current materialization's source hash when its recorded source
   revisions exactly match the migrated authoring resources.
5. Validate the normalized bytes against the current resource schema.
6. Atomically replace files and mark the project layout version 4 only after
   every file and metadata update succeeds.

A SQLite migration expands the `projects.layout_version` constraint to accept
version 4. Database columns and unrelated API/event contracts already using
`project_id` are unchanged.

## Validation

Tests must verify:

- all three resource schemas require `project_id` and reject `project`;
- Agent contracts hide `project_id`;
- Go and TypeScript serialization use `project_id`;
- layout v3 resources using `project` migrate in place to `project_id`;
- fresh materialization source hashes remain aligned with migrated bytes;
- current resources, snapshots, prompts and frontend consumers remain valid.
