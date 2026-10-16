## run_command

**Run shell commands to inspect files or make authorized project changes**.
Cases:
- Inspect files, search content, or examine changes more efficiently with a command.
- Transform, create, move or delete authorized project files, including batch changes and project-local dependency installation.
Rules:
- Commands use Bash with normal pipes, redirections, variables, substitutions and command composition; reading commands have no allowlist or sensitive-file approval.
- Use project-relative paths for writes. Local commands run in an isolated project copy; proposed changes require approval when enabled and are saved together only after validation.
- Writing outside the project, changing unauthorized pages or protected resources, global installation, and Git commit or push are prohibited.
- Use `edit_manifest`, `edit_design` and `edit_outline` to change their corresponding resources.
- Set `network` to `true` only when outbound network access is needed; approval applies to potential external effects as well as project writes. Run Git inspection separately with networking disabled.
- A failed, canceled or timed-out command does not save local changes; completed network effects cannot be rolled back.
Returns:
- `exit_code`: The shell's exit status.
- `stderr`: Captured standard error.
- `stdout`: Captured standard output.
- `feedback`: The user's reason for rejecting the request.
- `summary`: Returned on `status=refuse` to confirm that no project changes were saved or network request executed.
Errors:
- `COMMAND_INVALID`: The command is empty or has invalid shell syntax.
- `COMMAND_NOT_ALLOWED`: The command attempts a prohibited write or operation.
- `COMMAND_NOT_AVAILABLE`: The executable or required shell sandbox is unavailable.
- `COMMAND_EXIT_NONZERO`: The shell exited unsuccessfully.
- `COMMAND_TIMEOUT`: The command exceeded its execution deadline.
Args:
- command: Bash source to execute from the project directory; use project-relative paths for writes.
- network: Whether outbound network access is required; defaults to `false` and requires approval when enabled.
