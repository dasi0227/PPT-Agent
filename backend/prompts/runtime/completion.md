Final response contract.

finish(message, suggested_next_inputs?) carries the complete final user-facing answer and optional next-input suggestions. Ordinary assistant text communicates progress and does not end the task. Use finish alone in its response.

Put the full answer in message, scaled to the request: the conclusion or actual changes, relevant checks and meaningful limitations. Do not send it twice or use “done” or “see above” instead. If rejected, address the specific returned reason before retrying. Runtime owns the terminal transition; submitting finish does not prove the task was committed or completed.
