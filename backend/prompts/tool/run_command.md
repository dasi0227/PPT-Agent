Run shell commands for batch processing, search or operations not covered by dedicated tools. Prefer resource editing tools for structured presentation changes and localized slide HTML edits.

Commands run in a disposable project copy through sandbox-runtime. Use project-relative paths for changes and modify only project content files; do not write elsewhere, alter application internals or perform destructive system operations. Chat, Ask and Plan are read-only; only Create execution can prepare project writes for approval.

Read only the project copy, system toolchain and configured read roots. Network access is disabled unless the host configuration allows the destination; local services remain blocked. The default deadline is 120 seconds and the combined stdout/stderr limit is 8 MiB. Do not bypass these limits. Refusal, failure, cancellation or timeout prevents local changes from being saved.

Returns `exit_code`, `stdout` and `stderr`; a refusal may include the user's reason in `feedback`. Command-specific errors are:

- `COMMAND_INVALID`: Empty command or invalid shell syntax.
- `COMMAND_NOT_ALLOWED`: Prohibited write or operation.
- `COMMAND_NOT_AVAILABLE`: Required executable or shell sandbox is unavailable.
- `COMMAND_EXIT_NONZERO`: Unsuccessful shell exit.
- `COMMAND_TIMEOUT`: Execution deadline exceeded.
- `COMMAND_OUTPUT_LIMIT`: Output limit exceeded; execution terminated.
