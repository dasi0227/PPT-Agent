package com.dasi.ppt.agent.interfaces.controller.history.vo;

import com.dasi.ppt.agent.domain.agent.model.enumeration.CommandStatus;
import com.dasi.ppt.agent.domain.agent.model.enumeration.CommandType;
import com.fasterxml.jackson.annotation.JsonInclude;

import java.util.List;

public record CommandDetailVo(
        String revision,
        String operationId,
        String runId,
        CommandType type,
        CommandStatus status,
        long createdAt,
        Integer phase,
        CommandInputVo input,
        String lastSuccessId,
        CommandResultVo result,
        RunDetailVo.ExecutionFailureVo failure
) {

    @JsonInclude(JsonInclude.Include.NON_NULL)
    public record CommandInputVo(
            String text,
            String baseId,
            String feedback
    ) {
    }

    @JsonInclude(JsonInclude.Include.NON_NULL)
    public record CommandResultVo(
            String title,
            String content,
            Boolean changed,
            List<String> requests,
            Long beforeTokens,
            Long afterTokens,
            Long maxTokens,
            Boolean empty,
            String hash,
            String branch,
            List<String> items,
            Integer filesChanged,
            Integer insertions,
            Integer deletions,
            Long committedAt
    ) {
    }
}
