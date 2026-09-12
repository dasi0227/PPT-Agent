Return strict JSON only. Do not wrap it in Markdown.

Schema:

{
  "checks": [
    {
      "code": "REVIEW_PASS | REVIEW_SERVICE_UNAVAILABLE | REVIEW_LACK_INFO | REVIEW_QUALITY_POOR | REVIEW_INTENT_MISMATCH | REVIEW_EXECUTE_WRONG",
      "summary": ""
    }
  ]
}

Rules:
- checks must contain 1-5 items.
- If there are no issues, return exactly one REVIEW_PASS check.
- REVIEW_PASS must not appear with any other code.
- Do not include severity, decision, action, target, coverage, confidence, accepted, or issues fields.
- Summary must be concrete and useful and contain at least 20 characters, including REVIEW_PASS. Prefer 2-4 specific sentences when reporting a problem.
