package com.dasi.ppt.agent.interfaces.controller.session.vo;

import com.dasi.ppt.agent.domain.agent.model.enumeration.PlanStepStatus;
import com.fasterxml.jackson.annotation.JsonProperty;
import lombok.Builder;
import lombok.Getter;

import java.util.List;

@Builder
@Getter
public final class SessionStateVo {
    private String revision;
    private PlanVo plan;
    private ContextWindowVo context;
    private String activeRunId;
    @JsonProperty("active_command_ids")
    private List<String> activeActionIds;
    private List<String> suggestions;

    @Builder
    @Getter
    public static final class PlanVo {
        private String runId;
        private List<PlanStepVo> steps;
    }

    @Builder
    @Getter
    public static final class PlanStepVo {
        private String title;
        private PlanStepStatus status;
    }

    @Builder
    @Getter
    public static final class ContextWindowVo {
        private long current;
        private long total;
        private long thresholdMin;
        private long thresholdMax;
        private ContextDetailsVo details;
    }

    @Builder
    @Getter
    public static final class ContextDetailsVo {
        private List<TokenBreakdownItemVo> systemPrompt;
        private List<TokenBreakdownItemVo> runtime;
        private List<TokenBreakdownItemVo> chatHistory;
        private List<TokenBreakdownItemVo> readFile;
        private List<TokenBreakdownItemVo> other;
    }

    @Builder
    @Getter
    public static final class TokenBreakdownItemVo {
        private String name;
        private long tokens;
    }
}
