package com.dasi.ppt.agent.domain.agent.model.result;

import com.dasi.ppt.agent.domain.agent.model.enumeration.PreviewIssue;
import com.dasi.ppt.agent.domain.agent.model.enumeration.PreviewMismatch;
import com.dasi.ppt.agent.domain.agent.model.enumeration.PreviewStatus;
import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Getter;
import lombok.NoArgsConstructor;

@Builder
@Getter
@AllArgsConstructor
@NoArgsConstructor
public final class PreviewAssessment {
    private String slideId;
    private PreviewStatus status;
    private Scores scores;
    private PreviewMismatch mismatch;
    private PreviewIssue issue;
    private String reason;
    private String renderHash;
    private String screenshotId;

    @Builder
    @Getter
    @AllArgsConstructor
    @NoArgsConstructor
    public static final class Scores {
        private double contentCoverage;
        private double expressionClarity;
        private double requirementAdherence;
    }
}
