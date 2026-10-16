## run_command

**Run a restricted project-local command against durable project files without invoking a shell**.
Rules:
- Supports inspection and a restricted sed write form. Sensitive reads and permitted writes follow the high-risk approval setting. PPT resource writes remain subject to scope, current-source and domain validation.
Args:
- command: Project-local command, at most 4096 characters. Supported reads: pwd, ls, cat, head, tail, find, grep, rg, jq, stat, sed, wc, and git status/diff/log. Only | and && may join stages; each executable and its options must be allowed. Normal combined flags, attached/separate option values and -- are supported. Use project paths without .. and quote patterns, for example `rg -n -g '*.html' color . | head -n50`. File operands, option-loaded files and recursive symlink targets must remain within the project. Redirections, ||, semicolons, substitutions, variables, unquoted globs and helper execution are forbidden. Tail follow, find -exec/-delete, rg --pre/--search-zip, jq import/include/module paths and sed r/w/e are forbidden. The only write form is a standalone `sed -i '' 's/pattern/replacement/g' FILE` in execute mode, excluding .manifest.json, .design.json and .outline.json. Before a write, read the complete current target with a single-file cat; `read_resource` also supplies exact Spec JSON and HTML source. A parsed JSON object is insufficient. Writes retain command output verbatim and reject unseen or stale source. Each pipeline has a 10-second timeout and bounded stdout/stderr.
Returns:
- `exit_code`: Command exit status.
- `stderr`: Captured standard error; empty when none was captured.
- `stdout`: Captured standard output.
- `feedback`: Optional verbatim user feedback accompanying the decision; it grants no additional authorization.
- `summary`: Returned on `status=refuse` to acknowledge that the command was not executed.
Errors:
- `COMMAND_EXIT_NONZERO`: The command exited with a nonzero status; inspect `exit_code`, `stdout` and `stderr`.
- `COMMAND_FLAG_DENIED`: A flag or flag value is unsupported.
- `COMMAND_NOT_ALLOWED`: The executable or command form is outside the command policy.
- `COMMAND_NOT_AVAILABLE`: The allowed executable is unavailable.
- `COMMAND_OUTPUT_LIMIT`: Captured command output exceeded the allowed budget.
- `COMMAND_PARSE_INVALID`: Command quoting or tokenization is invalid.
- `COMMAND_PATH_INVALID`: A command path is invalid or unavailable.
- `COMMAND_SYNTAX_DENIED`: The command uses unsupported shell-like syntax.
- `COMMAND_TIMEOUT`: The command exceeded its execution deadline.
