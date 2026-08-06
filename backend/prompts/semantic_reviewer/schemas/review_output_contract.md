Return strict JSON only. Do not wrap it in Markdown.

Schema:

{
  "accepted": true,
  "confidence": 0.0,
  "summary": "",
  "coverage": [
    {
      "requirement_id": "req_01",
      "status": "satisfied | partial | missing | unverifiable",
      "evidence_refs": ["evidence-id-or-ref-id"],
      "reason": ""
    }
  ],
  "issues": [
    {
      "code": "REQUIREMENT_UNADDRESSED | QUALITY_RUBRIC_FAILED | FINAL_ANSWER_INCOMPLETE | USER_INTENT_MISMATCH | EVIDENCE_CONTRADICTION | CONTEXT_INSUFFICIENT",
      "severity": "warning | error | fatal",
      "requirement_id": "req_01",
      "target": {"type": "slide", "slide_id": "slide-01", "part": "html"},
      "summary": "",
      "required_action": {
        "tool": "read_ppt | write_ppt | edit_ppt | render_slide | search_refs | ask_user | finish",
        "target": {"type": "slide", "slide_id": "slide-01", "part": "html"}
      }
    }
  ]
}

When accepted is false, issues must contain at least one blocking issue.
