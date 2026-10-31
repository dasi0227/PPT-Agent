---
id: tool_run_command
desc: Execute commands in a disposable project copy.
---

Run Bash commands in a disposable project copy for search, batch processing or operations not covered by dedicated tools. Prefer dedicated editing tools for structured presentation changes and localized HTML edits. Use project-relative paths for changes. Do not modify protected application data or run Git commit or push operations. Submit commands that modify project files alone. Changes are applied only after successful execution, validation and user approval. A successful call returns `exit_code`, `stdout` and `stderr`. A refusal may include the user’s reason in `feedback`.
