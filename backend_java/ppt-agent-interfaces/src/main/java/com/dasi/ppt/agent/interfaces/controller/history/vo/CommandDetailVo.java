package com.dasi.ppt.agent.interfaces.controller.history.vo;

import com.dasi.ppt.agent.domain.agent.model.enumeration.CommandStatus;
import com.dasi.ppt.agent.domain.agent.model.enumeration.CommandType;
import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Getter;
import lombok.NoArgsConstructor;

import java.util.List;

@Builder
@Getter
@AllArgsConstructor
@NoArgsConstructor
public final class CommandDetailVo {
    private String revision;
    private String operationId;
    private String runId;
    private CommandType type;
    private CommandStatus status;
    private long createdAt;
    private Integer phase;
    private CommandInputVo input;
    private String lastSuccessId;
    private CommandResultVo result;
    private RunDetailVo.ExecutionFailureVo failure;

    @Builder
    @Getter
    @AllArgsConstructor
    @NoArgsConstructor
    public static final class CommandInputVo {
        private String text;
        private String baseId;
        private String feedback;
    }

    @Builder
    @Getter
    @AllArgsConstructor
    @NoArgsConstructor
    public static final class CommandResultVo {
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
