package com.dasi.ppt.agent.interfaces.controller.history.vo;

import com.dasi.ppt.agent.domain.agent.model.enumeration.CommandStatus;
import com.dasi.ppt.agent.domain.agent.model.enumeration.CommandType;
import com.dasi.ppt.agent.domain.agent.model.enumeration.InstructionDecision;
import com.dasi.ppt.agent.domain.agent.model.enumeration.InstructionReferenceType;
import com.dasi.ppt.agent.domain.agent.model.enumeration.InstructionStatus;
import com.dasi.ppt.agent.domain.agent.model.enumeration.RunStatus;
import com.dasi.ppt.agent.domain.project.model.enumeration.ChangeStatus;
import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Getter;
import lombok.NoArgsConstructor;

import java.util.List;

@Builder
@Getter
@AllArgsConstructor
@NoArgsConstructor
public final class HistoryPageVo {
    private String revision;
    private String nextBefore;
    private List<HistoryItemVo> items;

    @Builder
    @Getter
    @AllArgsConstructor
    @NoArgsConstructor
    public static final class HistoryItemVo {
        private String seq;
        private long createdAt;
        private RunHistoryVo run;
        private CommandHistoryVo command;
    }

    @Builder
    @Getter
    @AllArgsConstructor
    @NoArgsConstructor
    public static final class RunHistoryVo {
        private String id;
        private RunStatus status;
        private List<String> scope;
        private long activeDurationMs;
        private long totalTokens;
        private RunDetailVo.ExecutionFailureVo failure;
        private List<HistoryInstructionVo> instructions;
        private HistoryReplyVo response;
        private List<ChangeSummaryVo> changes;
    }

    @Builder
    @Getter
    @AllArgsConstructor
    @NoArgsConstructor
    public static final class CommandHistoryVo {
        private String id;
        private String operationId;
        private CommandType type;
        private CommandStatus status;
        private Integer phase;
        private RunDetailVo.ExecutionFailureVo failure;
        private String lastSuccessId;
        private CommandSummaryResultVo result;
    }

    @Builder
    @Getter
    @AllArgsConstructor
    @NoArgsConstructor
    public static final class HistoryInstructionVo {
        private String id;
        private String text;
        private long createdAt;
        private InstructionStatus instructionStatus;
        private InstructionDecision instructionDecision;
        private List<PublicReferenceVo> references;
    }

    @Builder
    @Getter
    @AllArgsConstructor
    @NoArgsConstructor
    public static final class HistoryReplyVo {
        private String id;
        private String text;
        private long createdAt;
    }

    @Builder
    @Getter
    @AllArgsConstructor
    @NoArgsConstructor
    public static final class PublicReferenceVo {
        private InstructionReferenceType type;
        private String assetId;
        private String slideId;
        private String attachmentId;
        private String originalName;
        private String selectionId;
        private Integer markerNo;
    }

    @Builder
    @Getter
    @AllArgsConstructor
    @NoArgsConstructor
    public static final class CommandSummaryResultVo {
        private String title;
        private Long beforeTokens;
        private Long afterTokens;
        private Long maxTokens;
        private Boolean empty;
        private String hash;
        private String branch;
        private Integer filesChanged;
        private Integer insertions;
        private Integer deletions;
    }

    @Builder
    @Getter
    @AllArgsConstructor
    @NoArgsConstructor
    public static final class ChangeSummaryVo {
        private RunChangesVo.ChangeTargetVo target;
        private ChangeStatus status;
        private Integer insertions;
        private Integer deletions;
    }
}
