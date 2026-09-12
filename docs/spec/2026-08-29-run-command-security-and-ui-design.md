# Run Command Restricted Execution Design

Date: 2026-08-29
Status: approved for implementation

## 1. Decision Summary

Add a model-visible tool named `run_command` for project-local inspection and
one narrowly defined write operation. It is not a general terminal and it does
not grant network access, package installation, process management, or
arbitrary shell execution.

The recommended first version uses three deterministic policy outcomes:

1. `allow`: the parsed command is provably read-only and project-local.
2. `confirm`: the command requests sensitive project content or is the one
   supported write form, `sed -i`.
3. `deny`: the command can write, execute another program indirectly, escape
   the project boundary, or uses syntax the policy does not understand.

User confirmation never upgrades a denied operation. Output redirection,
unrecognized `sed -i` scripts, and commands outside the allowlist remain denied
even when the user is willing to approve them.

This is a small policy sandbox, not a full hostile-code sandbox.

The following decisions are locked for the first implementation:

- Read commands are allowed or confirmed according to sensitivity.
- The only supported write operation is a validated `sed -i` substitution on
  one existing project-local text file in execute mode.
- Confirmed edits are staged into `RunSession`; the command must never mutate
  the baseline project file directly.
- Independent allowed read commands may run concurrently with a per-Run limit
  of 3. Approval-bound calls and denied calls never join a concurrent batch.
- Runtime emits `tool.started` immediately before process execution and
  `tool.completed` after termination.
- Frontend delays rendering the running row for 300 ms. Fast commands therefore
  appear only in their terminal state.
- The approved interaction reference is
  `docs/discuss/2026-08-29-run-command-state-preview.html`.
- The supported command set includes the confirmed preview examples: bounded
  `find` and the read-only Git subcommands `status`, `diff`, and `log`.

## 2. Why A Command Allowlist Is Not Enough

Checking only the first command name is unsafe:

- `echo value > file` writes even though `echo` itself is harmless.
- `sed -i`, `sed -I`, `w file`, and `s///w file` can write.
- `rg --pre` can launch another executable.
- `jq --slurpfile`, `--rawfile`, `--argfile`, and `-L` can read additional
  files that are not obvious from the final positional argument.
- `cat ../secret`, `rg pattern /`, and project symlinks can leave the project.
- `echo "$(curl ...)"` or backticks can execute a nested command before
  `echo`.
- a project-local executable can shadow a trusted binary when `PATH` includes
  the working directory.

Setting `cmd.Dir` to the project directory only chooses the initial working
directory. It does not prevent absolute paths, `..`, symlink traversal, config
file loading, or nested command execution.

## 3. Scope Of The First Version

### 3.1 Recommended commands

Supported:

```text
ls
cat
tail
head
find
grep
jq
rg
pwd
stat
sed
git status
git diff
git log
```

Optional addition:

```text
wc
```

`wc` is useful for measuring files without returning their full content.
`find` is retained because it is an established Agent discovery command, but
its predicates and traversal behavior are strictly limited.

`echo` is intentionally omitted. It does not retrieve project context and is
mainly useful with shell expansion or redirection, both of which this tool must
reject. If compatibility with common Agent habits is important, literal-only
`echo` can be added later without variable expansion or redirection.

This set is sufficient for the current goal:

- discover files and directories;
- search text and filenames;
- inspect full files or bounded sections;
- inspect JSON structurally;
- inspect metadata;
- compose bounded read pipelines.

### 3.2 Explicit non-goals

- no general-purpose terminal;
- no arbitrary executable paths;
- no file creation, deletion, or rename;
- no write operation except the validated single-file `sed -i` form;
- no Git mutation, package managers, interpreters, compilers, or build commands;
- no network commands;
- no background jobs or interactive TTY;
- no shell functions, aliases, environment mutation, or startup files;
- no persistent "always allow" approval;
- no Agent-authored approval classification;
- no container, VM, or remote execution environment in this iteration.

## 4. Model-Visible Contract

Keep the model contract intentionally small:

```json
{
  "command": "rg \"deck\" backend | head -n 40"
}
```

Schema:

```json
{
  "type": "object",
  "required": ["command"],
  "additionalProperties": false,
  "properties": {
    "command": {
      "type": "string",
      "minLength": 1,
      "maxLength": 4096,
      "description": "Run a restricted project-local command. Read operations are allowlisted; the only write form is a confirmed single-file sed -i substitution in execute mode."
    }
  }
}
```

Do not expose `cwd`, `env`, timeout, output limits, executable paths, approval
flags, or sandbox settings to the model. Runtime owns all of them.

There is a naming collision with the existing Go type `model.RunCommand`,
which is the task request contract. Keep the public tool name `run_command`,
but name the implementation by responsibility, for example:

```text
projectCommandTool
commandexec.Policy
commandexec.Executor
```

Do not rename the existing task contract only to accommodate this tool.

## 5. Parse First, Never Pass The String To A Shell

Use `mvdan.cc/sh/v3/syntax` to parse POSIX shell syntax into an AST. Do not use:

```go
exec.Command("sh", "-c", command)
```

The accepted AST subset is:

```text
simple command
simple command | simple command
pipeline && pipeline
```

Recommended limits:

- at most 4 pipeline stages;
- at most 4 `&&` groups;
- at most 128 arguments total;
- no command longer than 4096 bytes.

Every word must reduce to a static literal after quote removal. The following
AST forms are denied:

```text
$(...)
`...`
${...}
$VAR
*
?
[...]
>(...)
<(...) 
(...)
{...;}
name=value command
command &
command ;
command ||
multiline command lists
```

Globbing is unnecessary because `rg --glob` provides an explicit,
command-owned alternative.

### 5.1 Operators

`|`:

- supported;
- every stage is independently validated;
- stderr is not implicitly merged into stdout.

`&&`:

- supported;
- every group is independently validated before any group starts;
- execution stops at the first non-zero exit.

`||`, `;`, newline-separated lists, and background `&`:

- denied in the first version;
- they are detectable in the AST;
- they can be added later without changing the tool contract.

### 5.2 Redirection and heredocs

All redirection nodes are detected before execution.

Denied:

```text
>
>>
2>
2>>
<>
<
<<<
<<
<<-
```

`>` and `>>` are direct write paths. `<` and heredocs could theoretically be
made read-only, but they add parsing, size, and path-accounting complexity
without improving the current retrieval use cases. `cat`, `jq FILE`, and pipes
already provide simpler equivalents.

The command is either fully accepted before execution or fully rejected. A
compound command never partially runs before a later segment is found unsafe.

## 6. Command-Specific Policy

A generic path check is insufficient because each program assigns different
meaning to positional arguments and flags. Use one validator per command behind
a shared policy interface.

```go
type CommandPolicy interface {
    Validate(CommandNode, ProjectRoot) Decision
}

type Decision struct {
    Outcome       Outcome
    Mutates       bool
    Normalized    NormalizedCommand
    CommandHash   string
    ReasonCode    string
    PublicReason  string
}
```

The policy table is the single source of truth for:

- executable identity;
- allowed and denied flags;
- path-bearing arguments;
- cost limits;
- confirmation rules;
- normalized display text.

### 6.1 Baseline policy

| Command | Allowed use | Required restrictions |
|---|---|---|
| `pwd` | print project root identity | no arguments; public output should be `.` rather than an absolute server path |
| `ls` | list project files | deny absolute paths, `..`, `-R`, and symlink-following behavior |
| `cat` | read project files | file operands only; deny stdin marker `-`; enforce aggregate output limit |
| `head` | bounded file prefix | file operands must be local; clamp line/byte count |
| `tail` | bounded file suffix | deny `-f`, `-F`, `--follow`, `--retry`, and PID tracking; clamp count |
| `find` | bounded project discovery | local root only; clamp depth and result count; deny symlink following, `-exec`, `-execdir`, `-ok`, `-delete`, and file-output actions |
| `grep` | project text search | validate file roots and path-list flags; deny device files and external roots |
| `rg` | project text/file search | deny `--pre`, `--pre-glob`, `--follow`, external config, and external roots |
| `jq` | inspect JSON files/stdin | deny module loading, `-L`, and hidden file-loading flags unless separately validated |
| `stat` | inspect project file metadata | local paths only; normalize output that would expose the absolute root |
| `sed` | bounded stdout selection or one confirmed in-place substitution | read form accepts numeric print/range scripts; write form is restricted as defined below |
| `wc` | measure project files/stdin | local paths only |
| `git status` | inspect repository state | repository root must be inside the project; disable optional locks; bounded porcelain output |
| `git diff` | inspect tracked changes | inject `--no-ext-diff` and `--no-textconv`; deny output flags, external helpers, submodules, and paths outside the project |
| `git log` | inspect bounded history | clamp entry count; fixed non-interactive format; deny output, decoration helpers, mailmap loading outside the project, and unbounded traversal |

For all supported Git calls, Runtime uses the approved absolute Git binary and
injects a controlled configuration:

```text
GIT_OPTIONAL_LOCKS=0
GIT_PAGER=cat
PAGER=cat
GIT_EXTERNAL_DIFF=
GIT_CONFIG_NOSYSTEM=1
GIT_CONFIG_GLOBAL=<empty runtime file>
```

Runtime also injects safe command-owned flags instead of trusting model-supplied
configuration. Deny model-provided `-c`, `--config-env`, `--exec-path`,
`--git-dir`, `--work-tree`, pager overrides, external diff/text conversion,
submodule recursion, and every mutating Git subcommand.

### 6.2 `sed` decision

`sed 's/a/b/' file` changes the output stream, not the file. However, fully
proving every `sed` script read-only is disproportionate for this iteration.

Recommended first-version rule:

- allow `-n` with numeric address ranges and `p` or `q`;
- support exactly one write form: `sed -i '' 's/LITERAL/LITERAL/g?' FILE` on
  macOS or its normalized equivalent;
- require execute mode, exactly one existing regular UTF-8 text file, a maximum
  input size of 1 MiB, and `confirm`;
- require the write form to be the only command in the tool call: no pipeline,
  no `&&`, no redirection, and no multiple file operands;
- allow only a static substitution script with an optional numeric address and
  optional `g`; deny regex backreferences, command execution, file reads,
  file writes, and multiple script expressions;
- deny `w`, `r`, `e`, non-empty backup suffixes, and every other in-place form.

The confirmed write must run against isolated bytes, not the baseline file:

1. read the current content through `RunSession.Read`;
2. execute the approved absolute `sed` binary against a private temporary copy;
3. verify that only that temporary copy changed;
4. stage the resulting bytes into `RunSession.Write`;
5. report the staged `ArtifactChange`.

Add an `ArtifactProjectFile` artifact kind for this path. It participates in
`ChangeSet`, baseline validation, completion checks, commit, and rollback.
`ArtifactDerived` remains excluded from `ChangeSet`.

## 7. Project Boundary

### 7.1 Root definition

The authoritative root is the current project's `work_dir`, already passed to
Workflow as `RuntimeInput.ProjectDir`. It is not the repository root and is not
provided by the model.

At tool construction:

1. make the root absolute;
2. resolve root symlinks;
3. verify it exists and is a directory;
4. retain the canonical root for the whole call.

### 7.2 Path rules

For every path-bearing argument:

1. reject NUL;
2. reject absolute paths;
3. reject any cleaned path containing a parent escape;
4. resolve existing path symlinks;
5. resolve the nearest existing parent for a not-yet-existing path;
6. require the resolved target to equal the root or start with
   `root + path separator`;
7. reject special files, sockets, devices, and named pipes;
8. deny command flags that independently follow symlinks.

Prefer rejecting `../x` even when normalization would happen to land back
inside the root. The stricter rule is easier to explain and audit.

### 7.3 Important limitation

Application-level validation cannot literally guarantee that the process makes
no filesystem syscall outside the project. The executable and dynamic loader
must read system binaries and libraries. The practical promise should be:

> The command cannot read or write user/project data outside the selected
> project. Runtime dependencies may read approved system binaries and
> libraries.

There is also a small time-of-check/time-of-use window if an external process
can replace a validated file with a symlink before the command opens it. For a
single-user local development product this is acceptable in the first version.
It is not acceptable for hostile multi-tenant execution.

### 7.4 Sensitive reads

Keep one Runtime-owned sensitive-path policy. At minimum it covers `.env` and
`.env.*`, credential/secret files, private keys, `.netrc`, `.npmrc`, `.pypirc`,
SSH/AWS/Kubernetes credential locations, and Git configuration containing
remote credentials.

- An explicit content read of a sensitive path returns `confirm`.
- Broad content searches exclude sensitive paths by default and report that
  exclusion in the tool observation.
- The Agent may request an explicit sensitive path in a separate call, which
  then enters the approval flow.
- Approval applies only to the exact normalized command and paths for that call.
- Public event previews still pass through redaction after approval.

## 8. Process Execution

After parsing and policy validation, execute the normalized command graph with
`os/exec` directly.

Runtime settings:

```text
working directory: canonical project root
stdin: closed, except bounded bytes from an allowed previous pipeline stage
TTY: disabled
timeout: 10 seconds
stdout cap: 64 KiB
stderr cap: 16 KiB
pipeline stages: max 4
AND groups: max 4
concurrent run_command calls per Run: max 3
```

When a cap is reached:

- terminate the entire process group;
- return the captured prefix and `output_truncated=true`;
- use a typed error code rather than pretending the command completed.

Cancellation must kill the process group, not only the parent process.

Use approved absolute executable paths. Resolve them from server configuration
or controlled startup discovery, never from a `PATH` containing the project
directory.

Build an executable inventory at server startup. The inventory records the
approved absolute path and availability of each command. The model-visible tool
description should list the commands actually available on that host. A
missing binary returns `COMMAND_NOT_AVAILABLE`; Runtime must not silently fall
back to a shell, another executable, or a downloaded dependency.

Use a minimal environment:

```text
PATH=<approved system binary directories only>
HOME=<empty runtime directory>
XDG_CONFIG_HOME=<empty runtime directory>
RIPGREP_CONFIG_PATH=
LC_ALL=C
LANG=C
TERM=dumb
NO_COLOR=1
```

Do not inherit provider keys, database credentials, proxy variables, shell
startup configuration, or the server's complete environment.

Independent `run_command` calls from one model response may execute concurrently
only when every call has passed preflight with outcome `allow`. Preserve the
model call order in returned observations and timeline placement even when
processes finish out of order. A batch containing `confirm` must be rejected as
repairable unless that command is the only call in the response.

A mutating call is serialized against every other command call in the Run. It
must not overlap an active read process.

Emit a structured audit record for every decision and execution. Record run ID,
call ID, normalized command hash, safe display command, policy outcome and
reason code, validated relative target paths, approval decision, duration, exit
code, timeout/truncation flags, and staged change hashes. Do not record inherited
environment values, executable paths, full stdout/stderr, or secret content.

## 9. Is A Sandbox Necessary?

### 9.1 Recommended answer for the current product stage

A full container sandbox is not necessary for the restricted first version.
A small execution boundary is necessary. The one write form never runs against
the baseline project tree; it operates on a private temporary copy and stages
the result through `RunSession`.

That boundary consists of:

1. shell AST parsing;
2. a strict syntax subset;
3. per-command flag and operand policies;
4. canonical project path checks;
5. fixed executable paths;
6. a clean environment;
7. time, output, and process-group limits.

This is enough for a trusted local Agent operating on a user's own project,
provided the product describes it as a restricted project command runner rather
than arbitrary sandboxed code execution.

### 9.2 When an OS-level sandbox becomes necessary

Move to an OS isolation layer before any of the following:

- additional or direct-to-filesystem write-capable commands;
- interpreters such as Python, Node, Bash, or PowerShell;
- Git hooks, package managers, compilers, or user-provided executables;
- network-capable commands;
- untrusted repositories in a hosted or multi-tenant service;
- a security guarantee against symlink races or command implementation bugs.

At that point, use a platform abstraction backed by a container, namespace
sandbox, VM, or another maintained OS isolation mechanism. Do not grow the
command-policy layer into a substitute for process isolation.

## 10. Runtime Integration

`run_command` is a dynamically classified domain tool:

```text
read capability: project.command.read
write capability: project.command.edit
risk: low for normal reads, medium for sensitive reads, high for sed -i
read phases: chat, planning, executing
write phase: executing only
write session: required for sed -i
```

Add `CapabilityProjectCommandRead` and `CapabilityProjectCommandEdit` rather
than reusing generic capabilities. This keeps disclosure policy explicit.

Read commands inspect the project baseline. The `sed -i` write form reads its
single target through `RunSession`, so it sees earlier staged edits to that
file. This restriction must be documented in the tool description:

> Use `read_ppt` to inspect resources changed in the current Run. Use
> `run_command` for project files. Its only write form is a confirmed
> single-file `sed -i` substitution in execute mode.

The tool descriptor cannot use one static `ReadOnly` flag to decide batching.
Preflight must return `Mutates bool`. Runtime uses that per-call decision for
capability checks, approval, batching, and write-failure sequencing.

### 10.1 Preflight before idempotency and execution

Add an optional preflight interface:

```go
type PreflightTool interface {
    Preflight(context.Context, DomainToolInput) ToolDecision
}
```

Runtime performs preflight before emitting `tool.started`, acquiring the
execution idempotency record, or starting any command.

The current Runtime already has the required lifecycle shape: it emits
`tool.started` before `registry.Execute` and `tool.completed` afterward. The
implementation must extend the existing projector and public-event allowlist for
`run_command`, not create a second command-specific lifecycle.

For compound model responses:

- a `confirm` command must be the only tool call in that response;
- otherwise Runtime returns a repairable tool observation and asks the Agent
  to submit it separately;
- `deny` never pauses;
- an allowed non-mutating command can participate in normal read-only batching;
- a mutating command requires execute mode, `RunSession`, edit capability, and
  an `allow_once` answer before `tool.started`.

Event sequencing is deterministic:

```text
allowed read:
  tool.started -> tool.completed(status=completed|failed)

confirmed read or sed -i, allowed:
  command.permission_requested
  -> command.permission_answered(allow_once)
  -> tool.started
  -> tool.completed(status=completed|failed)

confirmed read or sed -i, denied:
  command.permission_requested
  -> command.permission_answered(deny)
  -> tool.completed(status=blocked)

policy denial:
  tool.completed(status=blocked)
```

Every event carries the existing `run_id`; tool events also retain the original
`call_id`. No `tool.started` event is emitted before approval or for a policy
denial.

### 10.2 Runtime-owned approval

Approval classification must not be supplied by the Agent.

Use a dedicated Runtime interaction:

```text
command.permission_requested
command.permission_answered
```

The request contains:

```json
{
  "schema_version": 3,
  "run_id": "run_123",
  "occurred_at": "2026-08-29T08:00:00Z",
  "interaction_id": "cmdperm_123",
  "call_id": "call_123",
  "command": "sed -i '' 's/target_type/deck_type/g' backend/config.go",
  "command_hash": "sha256:...",
  "reason_code": "PROJECT_FILE_EDIT",
  "reason": "该命令将修改项目文件，需要你的批准。"
}
```

The answer contains:

```json
{
  "interaction_id": "cmdperm_123",
  "call_id": "call_123",
  "command_hash": "sha256:...",
  "decision": "allow_once"
}
```

or:

```json
{
  "interaction_id": "cmdperm_123",
  "call_id": "call_123",
  "command_hash": "sha256:...",
  "decision": "deny"
}
```

Runtime checkpoints the pending normalized command, command hash, policy
version, mutation classification, target path, target preimage hash, call ID,
interaction ID, and resume phase. After restart, the same approval is
reconstructed before another model turn. Approval executes only the exact
normalized command against the same staged preimage hash.

Do not reuse `ask_user`. That control tool is Agent-authored and semantic;
security approval is Runtime-authored and policy-driven.

## 11. Result Contract

Model observation on success:

```json
{
  "ok": true,
  "summary": "command completed",
  "data": {
    "stdout": "...",
    "stderr": "",
    "exit_code": 0,
    "duration_ms": 18,
    "output_truncated": false
  },
  "changed_targets": [],
  "issues": [],
  "retryable": false
}
```

Read calls return `changed_targets=[]`. A successful confirmed `sed -i` returns
the staged project-file target and line diff statistics. Any baseline project
mutation before commit is a Runtime invariant violation and terminates the Run
as `run.error`.

Represent the staged edit as:

```text
ArtifactKind: project_file
ArtifactRef.Path: validated relative path
PublicTarget.Type: file
PublicTarget.Part: content
PublicTarget.DisplayName: validated relative path
```

Extend public target validation and history hydration for `file`. Do not expose
the absolute project root.

Recommended error codes:

```text
COMMAND_PARSE_INVALID
COMMAND_SYNTAX_DENIED
COMMAND_NOT_ALLOWED
COMMAND_NOT_AVAILABLE
COMMAND_FLAG_DENIED
COMMAND_PATH_OUTSIDE_PROJECT
COMMAND_PATH_INVALID
COMMAND_SENSITIVE_READ_DENIED
COMMAND_EXIT_NONZERO
COMMAND_TIMEOUT
COMMAND_OUTPUT_LIMIT
COMMAND_EXEC_FAILED
COMMAND_INVARIANT_VIOLATION
```

Policy denial and normal non-zero exits are tool failures, not Run terminal
errors. The Agent receives the structured observation and can choose a safer
command. An invariant violation is a Runtime error and terminates the Run.

## 12. Public Events And Frontend State

Extend `tool.started` and `tool.completed` with an optional safe command
projection:

```json
{
  "command": {
    "text": "rg \"deck\" backend | head -n 40",
    "status": "completed",
    "exit_code": 0,
    "duration_ms": 18,
    "output_truncated": false,
    "stdout_preview": "backend/agent/handler.go:42 ...",
    "stderr_preview": ""
  }
}
```

`tool.started` includes only `command.text`. Completion may include the other
fields. Public previews are independently bounded to 8 KiB for stdout and 4 KiB
for stderr, normalized to UTF-8, stripped of control sequences, and passed
through the existing public-text redaction layer. The model observation retains
the larger executor caps from section 8.

The command projection has an explicit terminal status: `completed`, `blocked`,
or `failed`. Authorization remains represented by the dedicated permission
events rather than overloading `tool.started`.

Never expose:

- absolute project paths;
- inherited environment;
- raw tool arguments beyond the normalized display command;
- full unbounded stdout or stderr;
- secrets;
- executable filesystem paths.

Frontend states:

| State | Presentation | Behavior |
|---|---|---|
| running | blue terminal icon and regular-weight title | render only after 300 ms; collapsed by default |
| completed | green terminal icon and regular-weight title | collapsed by default |
| blocked | amber terminal icon and regular-weight title | collapsed by default |
| failed | red terminal icon and regular-weight title | collapsed by default |
| waiting approval | inline permission card | exact command, reason, `允许一次`, `拒绝` |
| approval answered | compact permission row | records allowed or denied decision |

Ordinary tool rows have no state background. Expanded detail is one gray box
containing a bold command followed by plain stdout, stderr, or policy text. Do
not add labels such as `命令`, `结果`, or `错误`. Do not add nested cards,
result tables, badges, or summary metadata unless truncation must be disclosed.
For failures, show one concise human-readable error sentence after the command;
do not render an unformatted structured error object.

Three or more consecutive completed `run_command` entries may be collapsed into
one group. Grouping requires the same tool type and no error or warning. The
expanded children have no horizontal offset, and each child remains
independently expandable.

The group title uses the completed command count, for example `执行了 3 条命令`.
The group and every child default to collapsed. One or two commands are always
shown as separate rows.

Do not use the global backend-error Toast for these states. They belong to the
Run timeline and are part of normal Agent execution. Toast remains reserved for
request/network failures that prevent the interaction itself from completing.

## 13. Frontend Interaction Rules

Permission card:

- Runtime pauses the active clock and enters `waiting_input`.
- The composer follows the existing waiting behavior.
- `允许一次` and `拒绝` are explicit actions.
- No "always allow" option.
- Buttons lock while submitting.
- A stale or mismatched command hash shows the existing global error Toast and
  refreshes history.
- After submission, the card collapses to a compact answered row.
- The permission card supports both sensitive reads and the one confirmed write
  form. The approved visual fixture uses `sed -i`.

Blocked command:

- show the attempted normalized command;
- explain the exact unsupported construct;
- do not offer an approval button;
- use copy such as "当前仅允许读取，`>>` 会写入文件";
- allow the Agent to continue automatically.
- keep the row collapsed by default.

Failed command:

- distinguish policy denial from process failure;
- show the bold command and bounded stderr in one gray expandable box;
- mark timeout and output limit with their own copy;
- do not mark the whole Run failed unless the Runtime exhausts its normal
  consecutive-tool-failure budget.

## 14. Files Expected To Change

Backend:

```text
backend/internal/commandexec/
  ast.go
  policy.go
  policies_*.go
  paths.go
  executor.go
  errors.go
  *_test.go

backend/internal/workflow/default_tools.go
backend/internal/workflow/domain.go
backend/internal/workflow/tools.go
backend/internal/workflow/runtime.go
backend/internal/workflow/checkpoint.go
backend/internal/workflow/session.go
backend/internal/workflow/completion.go
backend/internal/workflow/public_events.go
backend/internal/model/public_event.go
backend/internal/model/event.go
backend/internal/run/input.go
backend/internal/run/engine.go
backend/internal/httpapi/run_handler.go
backend/internal/httpapi/router.go
backend/prompts/mode/execute.md
backend/go.mod
backend/go.sum
```

Frontend:

```text
frontend/src/api/types.ts
frontend/src/api/sse.ts
frontend/src/api/runs.ts
frontend/src/features/agent/eventReducer.ts
frontend/src/features/agent/historyHydrator.ts
frontend/src/features/agent/Timeline.tsx
frontend/src/features/agent/ActivityRows.tsx
frontend/src/features/agent/CommandPermissionCard.tsx
frontend/src/stores/runStore.ts
```

Tests must cover realtime SSE and history hydration with the same validators.

## 15. Verification Matrix

Parser and policy:

- every allowed simple command;
- bounded `find` and denied mutating/executing predicates;
- `git status`, `git diff`, and `git log` with injected safe configuration;
- every other Git subcommand and dangerous Git option denied;
- quoted literals and spaces;
- safe pipelines;
- safe `&&` lists;
- full preflight before partial execution;
- every denied redirection;
- substitutions, variables, globs, background jobs, `||`, and multiline lists;
- command-specific dangerous flags;
- oversized command and excessive stages.

Path boundary:

- relative project file;
- `..` escape;
- absolute path;
- project symlink to external file;
- symlinked directory escape;
- FIFO/device/socket;
- filenames beginning with `-`;
- path-list flags such as grep exclude files;
- concurrent symlink replacement documented as residual risk.

Execution:

- timeout kills the process group;
- output and stderr caps;
- non-zero exit;
- cancellation;
- missing approved binary;
- clean environment;
- no project-local binary shadowing.
- read concurrency capped at 3 with observation order preserved;
- mutating call serialized against all command execution;
- audit records contain decision metadata but no secrets or full output.

Approval:

- sensitive read pauses before `tool.started`;
- validated `sed -i` pauses before `tool.started`;
- write approval is available only in execute mode;
- allow once executes exact command;
- deny returns a tool observation without execution;
- stale hash and duplicate answer rejected;
- restart reconstructs pending approval;
- active duration excludes user waiting time.
- `sed -i` operates on a temporary copy and stages through `RunSession`;
- failed or canceled Runs leave the baseline file unchanged;
- successful completion commits the staged file with baseline validation;
- pipelines, `&&`, multiple targets, backups, and unsupported scripts remain
  denied for the write form.

Frontend:

- delayed running, complete, blocked, failed, waiting, allowed, and denied states;
- strict SSE validation and history hydration parity;
- keyboard focus and screen-reader labels;
- reduced motion;
- long command wrapping without layout shift;
- no absolute path or raw secret leakage.
- ordinary rows default collapsed and have no state background;
- detail uses one gray box with no redundant labels;
- grouped children align with the parent row without an offset.
- one or two commands remain separate; three or more eligible commands group;
- grouped title reports command count and all rows default collapsed;
- failed detail renders one readable sentence rather than structured error JSON.

## 16. Final Recommendation

Implement the policy sandbox now. Do not implement a full container sandbox
yet.

The minimal secure product boundary is:

```text
parse AST
-> validate syntax
-> validate command-specific flags
-> validate canonical project paths
-> classify allow / confirm / deny
-> execute normalized graph without a shell
-> enforce process and output limits
-> return structured observation
```

This satisfies the current retrieval goal without committing the project to a
general terminal architecture. The boundary remains intentionally replaceable
by an OS-level sandbox if the product later admits additional write commands,
interpreters, network access, or untrusted multi-tenant projects.

## 17. Implementation Handoff

Implement this document end to end in a clean session. Treat sections 1, 6.2,
10, 12, 13, and 15 as acceptance criteria. Do not expand write scope beyond the
single confirmed `sed -i` form. Do not add general shell access, Git mutation,
package managers, interpreters, or network commands. The three
explicitly supported read-only Git subcommands remain in scope.

Recommended implementation order:

1. Add the `commandexec` parser, normalized AST, policy validators, path checks,
   typed errors, and executor with focused unit tests.
2. Add `ArtifactProjectFile`, project-file target projection, and transactional
   staging through `RunSession`.
3. Register `run_command` with explicit read/edit capabilities and dynamic
   preflight classification.
4. Add preflight, approval checkpointing, permission request/answer APIs, and
   bounded concurrency to Runtime.
5. Extend public events, SSE validation, persistence, and history hydration with
   the safe command projection.
6. Implement the finalized timeline UI using the approved HTML reference.
7. Run backend tests, frontend tests, type checking, and the relevant build.

Staged commits are allowed and recommended. Each commit must be internally
coherent, preserve existing unrelated work, and pass the tests relevant to that
stage. A suitable sequence is:

1. parser, policies, path validation, executor, and unit tests;
2. project-file artifact staging, Runtime preflight, approval, and recovery;
3. public events, SSE/history hydration, and finalized frontend interaction;
4. integration tests, prompt policy updates, and final verification fixes.

Do not squash or amend earlier commits unless explicitly requested.
