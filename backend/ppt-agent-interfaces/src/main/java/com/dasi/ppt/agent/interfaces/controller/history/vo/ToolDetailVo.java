package com.dasi.ppt.agent.interfaces.controller.history.vo;

import com.dasi.ppt.agent.domain.agent.model.enumeration.InteractionType;
import com.dasi.ppt.agent.domain.agent.model.enumeration.PreviewIssue;
import com.dasi.ppt.agent.domain.agent.model.enumeration.PreviewMismatch;
import com.dasi.ppt.agent.domain.agent.model.enumeration.PreviewStatus;
import com.dasi.ppt.agent.domain.agent.model.enumeration.ToolResultStatus;
import com.dasi.ppt.agent.domain.asset.model.enumeration.AssetType;
import com.dasi.ppt.agent.domain.project.model.enumeration.DeckFileType;
import com.dasi.ppt.agent.domain.project.model.enumeration.ChangeStatus;
import com.dasi.ppt.agent.interfaces.controller.interaction.vo.InteractionFormVo;
import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Getter;
import lombok.NoArgsConstructor;
import tools.jackson.databind.JsonNode;

import java.util.List;

@Builder
@Getter
@AllArgsConstructor
@NoArgsConstructor
public final class ToolDetailVo {
    private JsonNode arguments;
    private ToolResultVo result;

    @Builder
    @Getter
    @AllArgsConstructor
    @NoArgsConstructor
    public static final class ToolResultVo {
        private ToolResultStatus status;
        private JsonNode content;
        private String text;
        private List<ToolChangedTargetVo> targets;
        private ToolImageVo image;
        private List<LoadedResourceVo> resources;
        private ToolCommandVo command;
        private ToolInteractionVo interaction;
        private PreviewAssessmentVo preview;
    }

    @Builder
    @Getter
    @AllArgsConstructor
    @NoArgsConstructor
    public static final class ToolChangedTargetVo {
        private DeckFileType part;
        private String path;
        private String slideId;
        private ChangeStatus status;
        private RunChangesVo.ChangeDiffVo diff;
    }

    @Builder
    @Getter
    @AllArgsConstructor
    @NoArgsConstructor
    public static final class ToolImageVo {
        private String source;
        private String url;
        private String slideId;
    }

    @Builder
    @Getter
    @AllArgsConstructor
    @NoArgsConstructor
    public static final class LoadedResourceVo {
        private AssetType type;
        private String id;
        private String name;
    }

    @Builder
    @Getter
    @AllArgsConstructor
    @NoArgsConstructor
    public static final class ToolCommandVo {
        private String text;
        private Integer exitCode;
        private Long durationMs;
        private String stdoutPreview;
        private String stderrPreview;
    }

    @Builder
    @Getter
    @AllArgsConstructor
    @NoArgsConstructor
    public static final class ToolInteractionVo {
        private InteractionType type;
        private InteractionFormVo.InteractionRequestVo request;
        private InteractionFormVo.InteractionAnswerVo answer;
        private List<String> appliedScope;
    }

    @Builder
    @Getter
    @AllArgsConstructor
    @NoArgsConstructor
    public static final class PreviewAssessmentVo {
        private String slideId;
        private PreviewStatus status;
        private PreviewScoresVo scores;
        private PreviewJudgesVo judges;
        private String reason;
    }

    @Builder
    @Getter
    @AllArgsConstructor
    @NoArgsConstructor
    public static final class PreviewScoresVo {
        private double contentCoverage;
        private double expressionClarity;
        private double requirementAdherence;
    }

    @Builder
    @Getter
    @AllArgsConstructor
    @NoArgsConstructor
    public static final class PreviewJudgesVo {
        private PreviewMismatch mismatch;
        private PreviewIssue issue;
    }
}
