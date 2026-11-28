package com.dasi.ppt.agent.interfaces.controller.interaction.vo;

import com.dasi.ppt.agent.domain.agent.model.enumeration.Decision;
import com.dasi.ppt.agent.domain.agent.model.enumeration.InteractionType;
import com.dasi.ppt.agent.domain.project.model.enumeration.DeckFileType;
import com.fasterxml.jackson.annotation.JsonInclude;
import com.fasterxml.jackson.annotation.JsonUnwrapped;
import lombok.Builder;
import lombok.Getter;
import tools.jackson.databind.JsonNode;

import java.util.List;

@Builder
@Getter
public final class InteractionFormVo {
    private InteractionType type;
    private String version;
    @JsonUnwrapped
    private InteractionRequestVo request;

    @Getter
    public static final class QuestionVo {
        private String id;
        private String question;
        private String reason;
        private List<QuestionOptionVo> options;
    }

    @Getter
    public static final class QuestionOptionVo {
        private String id;
        private String label;
        private String description;
    }

    @Getter
    public static final class PlanFormVo {
        private String content;
        private List<String> steps;
    }

    @Getter
    public static final class PermissionFormVo {
        private String command;
    }

    @Getter
    public static final class ScopeFormVo {
        private String reason;
        private List<String> current;
        private List<String> proposed;
    }

    @Getter
    @JsonInclude(JsonInclude.Include.NON_NULL)
    public static final class InteractionRequestVo {
        private List<QuestionVo> questions;
        private PlanFormVo plan;
        private PermissionFormVo permission;
        private ScopeFormVo scope;
        private DeckFileType resource;
        private JsonNode base;
        private JsonNode proposal;
    }

    @Builder
    @Getter
    public static final class InteractionAnswerVo {
        private List<QuestionAnswerVo> answers;
        private Decision decision;
        private String feedback;
    }

    @Builder
    @Getter
    public static final class QuestionAnswerVo {
        private String questionId;
        private String selectedOptionId;
        private String customText;
        private Boolean skipped;
    }
}
