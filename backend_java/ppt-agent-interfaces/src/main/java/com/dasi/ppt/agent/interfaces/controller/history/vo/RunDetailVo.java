package com.dasi.ppt.agent.interfaces.controller.history.vo;

import com.dasi.ppt.agent.domain.agent.model.enumeration.InteractionType;
import com.dasi.ppt.agent.domain.agent.model.enumeration.MessageType;
import com.dasi.ppt.agent.domain.agent.model.enumeration.RunMode;
import com.dasi.ppt.agent.domain.agent.model.enumeration.RunStatus;
import com.dasi.ppt.agent.domain.agent.model.enumeration.ToolResultStatus;
import com.dasi.ppt.agent.domain.shared.enumeration.RunActivity;
import com.fasterxml.jackson.annotation.JsonInclude;

import java.util.List;

public record RunDetailVo(
        RunStatus status,
        RunMode mode,
        String model,
        List<String> scope,
        String revision,
        RunUsageVo usage,
        long activeDurationMs,
        Long activeSince,
        long createdAt,
        long updatedAt,
        ExecutionFailureVo failure,
        RunActivity activity,
        String commentary,
        List<PendingInteractionVo> pending,
        List<RunMessageVo> messages
) {

    @JsonInclude(JsonInclude.Include.NON_NULL)
    public record RunMessageVo(
            String id,
            String seq,
            MessageType type,
            long createdAt,
            String text,
            String name,
            RunChangesVo.ChangeTargetVo target,
            String toolCallId,
            ToolResultStatus status,
            Boolean hasDetail
    ) {
    }

    public record RunUsageVo(
            long inputTokens,
            long outputTokens,
            long totalTokens,
            long callCount
    ) {
    }

    public record PendingInteractionVo(
            String toolCallId,
            InteractionType type,
            String version
    ) {
    }

    public record ExecutionFailureVo(String code, String message) {
    }
}
