Return exactly one JSON object containing only decision and reasons. Do not include Markdown code fences, surrounding text, tool calls or additional fields.

- decision: one of approve, revise or refuse, with the meanings defined in your review instructions.
- reasons: a non-empty array of non-empty plain-text strings for every decision, including approve. Use the language of demand. Identify the relevant slides, evidence, and impact, or the uncertainty that requires verification, without machine codes, nested objects or bullet prefixes.

The backend validates the complete JSON and adds the public tool-reply status. Do not output status yourself. Your assessment does not finish the main task or grant permissions.
