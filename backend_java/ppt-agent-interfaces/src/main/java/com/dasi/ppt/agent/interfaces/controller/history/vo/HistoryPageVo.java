package com.dasi.ppt.agent.interfaces.controller.history.vo;

import com.dasi.ppt.agent.domain.agent.model.enumeration.CommandStatus;
import com.dasi.ppt.agent.domain.agent.model.enumeration.CommandType;
import com.dasi.ppt.agent.domain.agent.model.enumeration.InstructionDecision;
import com.dasi.ppt.agent.domain.agent.model.enumeration.InstructionReferenceType;
import com.dasi.ppt.agent.domain.agent.model.enumeration.InstructionStatus;
import com.dasi.ppt.agent.domain.agent.model.enumeration.RunStatus;
import com.dasi.ppt.agent.domain.project.model.enumeration.ChangeStatus;
import com.fasterxml.jackson.annotation.JsonInclude;

import java.util.List;

public record HistoryPageVo(
        String revision,
        String nextBefore,
        List<HistoryItemVo> items
) {

    @JsonInclude(JsonInclude.Include.NON_NULL)
    public record HistoryItemVo(
            String seq,
            long createdAt,
            RunHistoryVo run,
            CommandHistoryVo command
    ) {
    }

    public record RunHistoryVo(
            String id,
            RunStatus status,
            List<String> scope,
            long activeDurationMs,
            long totalTokens,
            RunDetailVo.ExecutionFailureVo failure,
            List<HistoryInstructionVo> instructions,
            HistoryReplyVo response,
            List<ChangeSummaryVo> changes
    ) {
    }

    public record CommandHistoryVo(
            String id,
            String operationId,
            CommandType type,
            CommandStatus status,
            Integer phase,
            RunDetailVo.ExecutionFailureVo failure,
            String lastSuccessId,
            CommandSummaryResultVo result
    ) {
    }

    public record HistoryInstructionVo(
            String id,
            String text,
            long createdAt,
            InstructionStatus instructionStatus,
            InstructionDecision instructionDecision,
            List<PublicReferenceVo> references
    ) {
    }

    public record HistoryReplyVo(
            String id,
            String text,
            long createdAt
    ) {
    }

    @JsonInclude(JsonInclude.Include.NON_NULL)
    public record PublicReferenceVo(
            InstructionReferenceType type,
            String assetId,
            String slideId,
            String attachmentId,
            String originalName,
            String selectionId,
            Integer markerNo
    ) {
    }

    @JsonInclude(JsonInclude.Include.NON_NULL)
    public record CommandSummaryResultVo(
            String title,
            Long beforeTokens,
            Long afterTokens,
            Long maxTokens,
            Boolean empty,
            String hash,
            String branch,
            Integer filesChanged,
            Integer insertions,
            Integer deletions
    ) {
    }

    public record ChangeSummaryVo(
            RunChangesVo.ChangeTargetVo target,
            ChangeStatus status,
            Integer insertions,
            Integer deletions
    ) {
    }
}
