# Run Command Contract Design

Status: implemented

## Purpose

Replace the current `WorkSpec` vocabulary with a smaller and explicit command
contract:

```json
{
  "scope": {
    "artifact": "ppt",
    "level": "slide",
    "slide_id": "sli_8n4wcp"
  },
  "intent": "execute",
  "instruction": "优化当前页面",
  "options": {
    "language": "zh-CN",
    "range": "9-15"
  }
}
```

The contract separates three responsibilities:

- `scope` declares the maximum resource boundary;
- `intent` declares the maximum interaction authority;
- the Agent decides execution orchestration inside the Harness Loop.

There is no derived Strategy layer.

## Final Domain Types

```go
type Artifact string

const (
    ArtifactSpec Artifact = "spec"
    ArtifactPPT  Artifact = "ppt"
)

type ScopeLevel string

const (
    ScopeSlide ScopeLevel = "slide"
    ScopeDeck  ScopeLevel = "deck"
)

type RunIntent string

const (
    IntentTalk    RunIntent = "talk"
    IntentAsk     RunIntent = "ask"
    IntentPlan    RunIntent = "plan"
    IntentExecute RunIntent = "execute"
)

type RunScope struct {
    Artifact Artifact   `json:"artifact"`
    Level    ScopeLevel `json:"level"`
    SlideID  string     `json:"slide_id,omitempty"`
}

type RunOptions struct {
    Language RunLanguage `json:"language,omitempty"`
    Range    SlideRange  `json:"range,omitempty"`
}

type RunCommand struct {
    Scope       RunScope  `json:"scope"`
    Intent      RunIntent `json:"intent"`
    Instruction string    `json:"instruction"`
    Options     RunOptions `json:"options,omitempty"`
}
```

The `RunInteraction` wrapper is removed. `RunIntent` is a caller-supplied
authorization value, not an Agent or Runtime decision.

## Validation

`RunCommand.Validate` enforces:

- `scope.artifact` is `spec` or `ppt`;
- `scope.level` is `slide` or `deck`;
- slide scope requires a stable `slide_id`;
- deck scope forbids `slide_id`;
- `slide_id = "current"` is forbidden;
- `intent` is `talk`, `ask`, `plan` or `execute`;
- `instruction` is non-empty;
- `options.language`, when present, is `zh-CN` or `en-US`;
- `options.range`, when present, is `5-8`, `9-15`, `16-25` or `26+`;
- `options.range` is only valid for deck scope.

Both options are optional. An omitted language inherits the current Outline
language. An omitted range imposes no run-specific deck-size requirement.

## Option Semantics

### Language

`language` is the language requested for content produced by this command.

- For an empty deck, the Agent writes the selected value into
  `outline.language`.
- For an existing deck, the current `outline.language` remains the default.
- If the option conflicts with the current Outline and the instruction does
  not explicitly request translation or language switching, the Agent asks the
  user instead of silently changing the deck language.
- For deck execute runs, completion requires the resulting
  `outline.language` to match an explicitly supplied language.
- Slide-scoped language quality remains a semantic review concern; Runtime does
  not attempt rule-based language detection.

### Range

`range` describes an inclusive desired deck-size bucket:

```text
5-8   -> 5 through 8 slides
9-15  -> 9 through 15 slides
16-25 -> 16 through 25 slides
26+   -> 26 or more slides
```

For deck execute runs, Completion Gate rejects a final Outline whose
`outline_order` count is outside the selected bucket.

## Resource Access

The internal `workflow.Scope` wrapper is removed because it contains no state
other than `RunScope`.

Workflow owns pure authorization functions:

```go
func AllowsRead(scope model.RunScope, resource Resource) bool
func AllowsWrite(scope model.RunScope, resource Resource) bool
func AllowsArtifact(scope model.RunScope, artifact ArtifactRef) bool
```

The functions do not grant authority. They only project the caller-declared
`RunScope` onto a concrete resource access decision.

`spec` scope cannot read or write slide HTML. `ppt` scope can access HTML inside
the declared slide or deck boundary. Deck-level Outline and Design remain
readable for slide-scoped work.

## Runtime and Context

All runtime structures use `RunCommand` directly:

- `model.Run.Command`
- `model.CreateRunParams.Command`
- `ContextRequest.Command`
- `ContextPack.Command` serialized as `run_command`
- `RetrievalQuery.Command`
- `SemanticReviewInput.Command`
- `RequirementLedger` source

ContextPack schema version becomes `2.0`.

Context profiles become:

```text
spec/deck
spec/slide
ppt/deck
ppt/slide
```

The required context segment is renamed from `work_spec` to `run_command`.
Target content inside ContextPack remains `target_context`; it represents the
loaded resource, not the command boundary.

## REST Contract

Create Run request:

```json
{
  "client_request_id": "req_8n4wcp",
  "model": "kimi-k3",
  "scope": {
    "artifact": "ppt",
    "level": "slide",
    "slide_id": "sli_8n4wcp"
  },
  "intent": "execute",
  "instruction": "优化当前页面",
  "options": {
    "language": "zh-CN",
    "range": "9-15"
  }
}
```

Run response exposes `scope` and `intent`; it no longer exposes `target` or the
single-field `interaction` object.

The server and frontend move atomically to the new request contract. The REST
endpoint does not accept both old and new field names, avoiding ambiguous
precedence.

## Public Events and History

Public event schema version becomes `3`.

New `run.started` payload:

```json
{
  "schema_version": 3,
  "run_id": "run_01",
  "occurred_at": "2026-08-10T00:00:00Z",
  "scope": {
    "artifact": "ppt",
    "level": "slide",
    "slide_id": "sli_8n4wcp"
  },
  "intent": "execute",
  "user_input": "优化当前页面"
}
```

New thread `user_turn` history entries persist `scope` and `intent`.

There is no runtime compatibility layer. The frontend accepts only Public
Event v3 and history entries written with the new contract. REST requests with
legacy fields fail validation. No component dual-reads or dual-writes the old
shape.

## SQLite Migration

The `runs` table is rebuilt with:

```text
scope_artifact
scope_level
scope_slide_id
intent
run_command_json
```

The migration removes:

```text
target_artifact
target_level
target_slide_id
interaction_intent
work_spec_json
```

Existing rows are transformed:

- `target` becomes `scope`;
- `interaction.intent` becomes top-level `intent`;
- artifact `presentation` becomes `ppt`;
- `theme_id` is removed;
- `desired_slide_count` maps to the closest new range bucket;
- `language` is retained;
- the transformed JSON is stored in `run_command_json`.

Historical public events and thread history are not rewritten and are not part
of the supported runtime contract.

## Frontend Contract

TypeScript uses:

```ts
type Artifact = 'spec' | 'ppt'
type ScopeLevel = 'slide' | 'deck'
type RunIntent = 'talk' | 'ask' | 'plan' | 'execute'
interface RunScope { artifact: Artifact; level: ScopeLevel; slide_id?: string }
interface RunOptions { language?: 'zh-CN' | 'en-US'; range?: '5-8' | '9-15' | '16-25' | '26+' }
```

Run sessions store `scope` and `intent`. Composer shortcuts update those fields
directly. User-facing labels may continue to use the Chinese word “目标” where
that is clearer UI copy; protocol and type names use Scope.

## Error Handling

Invalid commands return the existing create-run validation error envelope with
specific validation details. Unsupported legacy REST payloads fail validation
instead of being silently inferred.

Completion issues added by command options:

```text
RUN_LANGUAGE_UNSATISFIED
RUN_RANGE_UNSATISFIED
```

They include an actionable summary and do not expand scope automatically.

## Non-Goals

- No Strategy replacement is introduced.
- No theme selection remains in RunOptions.
- No UI for editing language or range is added in this change.
- No natural-language keyword rule is added to detect translation intent.
- No historical event or history file is rewritten.

## Self-Review

- RunCommand is the only task contract across REST, storage, Context and
  Workflow.
- RunScope is data; read/write authorization remains explicit Runtime policy.
- RunIntent grants authority without reintroducing Strategy routing.
- RunOptions contains no project-owned theme state or exact slide count.
- Public Event v3 and ContextPack v2 use one write/read shape with no runtime
  compatibility branch.
- SQLite migration ends with only new columns and new JSON.
- Completion checks enforce scope, language and range without semantic keyword
  routing.

Result: no unresolved contract conflict or implementation blocker remains.

## Verification

Coverage must include:

- command validation and enum rejection;
- slide/deck scope invariants;
- SQLite row and JSON migration;
- current Run store round-trip;
- v2 SSE rejection and strict v3 history hydration;
- v3 public event validation;
- spec versus PPT read/write authorization;
- ContextPack v2 serialization and profile selection;
- language/range Completion Gate behavior;
- create/get Run API shape;
- frontend request construction, hydration and retry;
- TypeScript, frontend tests and production build.
