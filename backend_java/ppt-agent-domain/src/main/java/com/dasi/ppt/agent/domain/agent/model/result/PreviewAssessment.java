package com.dasi.ppt.agent.domain.agent.model.result;

import com.dasi.ppt.agent.domain.agent.model.enumeration.PreviewIssue;
import com.dasi.ppt.agent.domain.agent.model.enumeration.PreviewMismatch;
import com.dasi.ppt.agent.domain.agent.model.enumeration.PreviewStatus;

public record PreviewAssessment(
        String slideId,
        PreviewStatus status,
        Scores scores,
        PreviewMismatch mismatch,
        PreviewIssue issue,
        String reason,
        String renderHash,
        String screenshotId
) {

    public record Scores(
            double contentCoverage,
            double expressionClarity,
            double requirementAdherence
    ) {
    }
}
