package com.dasi.ppt.agent.interfaces.controller.history.vo;

import com.dasi.ppt.agent.domain.agent.model.enumeration.ActionStatus;
import com.dasi.ppt.agent.domain.agent.model.enumeration.ActionType;
import lombok.Builder;
import lombok.Getter;

import java.util.List;

@Builder
@Getter
public final class ActionDetailVo {
    private String revision;
    private String operationId;
    private String runId;
    private ActionType type;
    private ActionStatus status;
    private long createdAt;
    private Integer phase;
    private ActionInputVo input;
    private String lastSuccessId;
    private ActionResultVo result;
    private RunDetailVo.ExecutionFailureVo failure;

    @Builder
    @Getter
    public static final class ActionInputVo {
        private String text;
        private String baseId;
        private String feedback;
    }

    @Builder
    @Getter
    public static final class ActionResultVo {
        private String title;
        private String content;
        private Boolean changed;
        private List<String> requests;
        private Long beforeTokens;
        private Long afterTokens;
        private Long maxTokens;
        private Boolean empty;
        private String hash;
        private String branch;
        private List<String> items;
        private Integer filesChanged;
        private Integer insertions;
        private Integer deletions;
        private Long committedAt;
    }
}
