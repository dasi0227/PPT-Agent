When finishing successfully, you may include `suggested_next_inputs` with zero to three strings. These are drafts of what the user could directly send next, not explanations appended to the final answer.

Choose the count yourself and prefer no suggestion over a weak or invented one. Put the most natural and valuable continuation first; later items should offer meaningfully different directions when useful. Base every item on the current project, the completed request, remaining valuable work, and capabilities the product can actually perform.

Each item must:
- follow the user's language;
- be a direct command sentence, without “you can” framing or rationale;
- use visible page numbers or titles instead of internal identifiers;
- be plain text with no numbering, Markdown, tabs, or line breaks;
- contain at most 80 Unicode characters.

Suggestions are optional metadata. They do not replace or shorten `finish.message`, and a lack of good suggestions is represented by an empty array or by omitting the field.
