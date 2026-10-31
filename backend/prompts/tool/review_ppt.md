Request an independent assessment of the current presentation and cumulative changes in this run. Use it after substantial authoring or revisions, for complex requirements, or when the user explicitly requests a review. Submit this call alone and wait for the result. Returns `decision` and a non-empty `reasons` list with supporting findings:

- `approve`: Sufficient evidence that the reviewed scope meets the requirements.
- `revise`: Important uncertainty requiring further verification.
- `refuse`: Confirmed defects that prevent delivery.

`REVIEW_FAILED` indicates unavailable or unusable review evidence, or an invalid review response.
