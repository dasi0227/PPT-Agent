package com.dasi.ppt.agent.interfaces.controller.history.vo;

import com.dasi.ppt.agent.domain.agent.model.enumeration.InteractionType;
import com.dasi.ppt.agent.domain.agent.model.enumeration.MessageType;
import com.dasi.ppt.agent.domain.agent.model.enumeration.RunMode;
import com.dasi.ppt.agent.domain.agent.model.enumeration.RunStatus;
import com.dasi.ppt.agent.domain.agent.model.enumeration.ToolResultStatus;
import com.dasi.ppt.agent.domain.agent.model.enumeration.RunActivity;
import com.fasterxml.jackson.annotation.JsonInclude;
import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Getter;
import lombok.NoArgsConstructor;

import java.util.List;

@Builder
@Getter
@AllArgsConstructor
@NoArgsConstructor
public final class RunDetailVo {
    private RunStatus status;
    private RunMode mode;
    private String model;
    private List<String> scope;
    private String revision;
    private RunUsageVo usage;
    private long activeDurationMs;
    private Long activeSince;
    private long createdAt;
    private long updatedAt;
    private ExecutionFailureVo failure;
    private RunActivity activity;
    private String commentary;
    private List<PendingInteractionVo> pending;
    private List<RunMessageVo> messages;

    @Builder
    @JsonInclude(JsonInclude.Include.NON_NULL)
    @Getter
    @AllArgsConstructor
    @NoArgsConstructor
    public static final class RunMessageVo {
        private String id;
        private String seq;
        private MessageType type;
        private long createdAt;
        private String text;
        private String name;
        private RunChangesVo.ChangeTargetVo target;
        private String toolCallId;
        private ToolResultStatus status;
        private Boolean hasDetail;
    }

    @Builder
    @Getter
    @AllArgsConstructor
    @NoArgsConstructor
    public static final class RunUsageVo {
        private long inputTokens;
        private long outputTokens;
        private long totalTokens;
        private long callCount;
    }

    @Builder
    @Getter
    @AllArgsConstructor
    @NoArgsConstructor
    public static final class PendingInteractionVo {
        private String toolCallId;
        private InteractionType type;
        private String version;
    }

    @Builder
    @Getter
    @AllArgsConstructor
    @NoArgsConstructor
    public static final class ExecutionFailureVo {
        private String code;
        private String message;
    }
}
