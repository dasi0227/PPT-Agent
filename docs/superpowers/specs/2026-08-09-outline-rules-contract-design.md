# Outline Rules Contract Design

## Decision

`outline.json` promotes deck-wide rules to two required top-level fields:

```json
{
  "requirements": [
    "覆盖企业客户续约趋势",
    "保持正式汇报语气",
    "总页数不超过 12 页"
  ],
  "prohibitions": [
    "虚构未提供的数据",
    "使用赛博朋克视觉风格"
  ]
}
```

This supersedes the nested `constraints` object defined in the earlier Outline
contract.

## Semantics

- `requirements` contains content, style, quantity and business rules that the
  deck must satisfy.
- `prohibitions` contains content, expression, style and behavior that must not
  appear.
- Both fields are required and may contain an empty array.
- Each item is one non-empty, independently checkable natural-language rule.
- Neither field is Runtime-owned; both are visible and writable in the Agent
  contract.

The top-level placement makes these rules first-class deck semantics alongside
`goal`, `audience` and `positioning`. The removed `constraints` wrapper did not
add ownership, lifecycle or validation semantics.

## Migration

The filesystem layout version advances from 4 to 5. Existing Outline files and
Outline snapshots migrate atomically:

```text
constraints.must_include
constraints.style_limits
constraints.content_limits
  -> requirements

constraints.must_avoid
  -> prohibitions
```

Migration preserves source order, removes duplicate strings and deletes the
obsolete `constraints` object. Existing top-level `requirements` or
`prohibitions` take precedence if already present.

Because Outline bytes change, migration updates a valid current
`materialization.json.source.hash` only when its stored revisions and old
source hash exactly match the pre-migration files. Stale or inconsistent
materialization records are not promoted to fresh.

## Validation

Coverage includes:

- Schema acceptance of the two top-level arrays.
- Schema rejection of the obsolete `constraints` field.
- v4 current Outline and Outline snapshot migration.
- preservation of rule ordering and polarity.
- materialization source-hash alignment.
- Go and TypeScript contract compilation.
