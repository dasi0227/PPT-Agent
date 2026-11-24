package com.dasi.ppt.agent.interfaces.controller.interaction.vo;

import com.dasi.ppt.agent.domain.agent.model.enumeration.InteractionType;
import com.dasi.ppt.agent.domain.project.model.enumeration.ChangePart;
import com.dasi.ppt.agent.interfaces.controller.history.vo.ToolDetailVo;
import com.fasterxml.jackson.annotation.JsonUnwrapped;
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
public final class InteractionFormVo {
    private InteractionType type;
    private String version;
    @JsonUnwrapped
    private InteractionRequestVo request;

    @Builder
    @Getter
    @AllArgsConstructor
    @NoArgsConstructor
    public static final class QuestionVo {
        private String id;
        private String question;
        private String reason;
        private List<QuestionOptionVo> options;
    }

    @Builder
    @Getter
    @AllArgsConstructor
    @NoArgsConstructor
    public static final class QuestionOptionVo {
        private String id;
        private String label;
        private String description;
    }

    @Builder
    @Getter
    @AllArgsConstructor
    @NoArgsConstructor
    public static final class PlanFormVo {
        private String content;
        private List<String> steps;
    }

    @Builder
    @Getter
    @AllArgsConstructor
    @NoArgsConstructor
    public static final class PermissionFormVo {
        private String command;
        private String stdout;
        private String stderr;
        private List<ToolDetailVo.ToolChangedTargetVo> changes;
    }

    @Builder
    @Getter
    @AllArgsConstructor
    @NoArgsConstructor
    public static final class ScopeFormVo {
        private String reason;
        private List<String> current;
        private List<String> proposed;
    }

    @Builder
    @Getter
    @AllArgsConstructor
    @NoArgsConstructor
    public static final class InteractionRequestVo {
        private List<QuestionVo> questions;
        private PlanFormVo plan;
        private PermissionFormVo permission;
        private ScopeFormVo scope;
        private ChangePart resource;
        private JsonNode base;
        private JsonNode proposal;
    }

    @Builder
    @Getter
    @AllArgsConstructor
    @NoArgsConstructor
    public static final class InteractionAnswerVo {
        private List<QuestionAnswerVo> answers;
        private String decision;
        private String feedback;
    }

    @Builder
    @Getter
    @AllArgsConstructor
    @NoArgsConstructor
    public static final class QuestionAnswerVo {
        private String questionId;
        private String selectedOptionId;
        private String customText;
        private Boolean skipped;
    }
}
