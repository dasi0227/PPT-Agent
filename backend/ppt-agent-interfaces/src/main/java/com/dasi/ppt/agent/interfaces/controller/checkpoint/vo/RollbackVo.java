package com.dasi.ppt.agent.interfaces.controller.checkpoint.vo;

import com.dasi.ppt.agent.domain.agent.model.enumeration.InstructionReferenceType;
import com.dasi.ppt.agent.domain.agent.model.enumeration.RunMode;
import com.dasi.ppt.agent.interfaces.controller.execution.dto.AppendRunDto;
import lombok.Builder;
import lombok.Getter;

import java.util.List;

@Builder
@Getter
public final class RollbackVo {
    private String revision;
    private RollbackInputVo input;

    @Builder
    @Getter
    public static final class RollbackInputVo {
        private String text;
        private List<FullReferenceVo> references;
        private RunMode mode;
        private String model;
        private InitialScopeVo scope;
    }

    @Builder
    @Getter
    public static final class InitialScopeVo {
        private String currentSlideId;
        private List<String> slideIds;
        private List<String> chapterIds;
        private List<String> sectionIds;
    }

    @Builder
    @Getter
    public static final class FullReferenceVo {
        private InstructionReferenceType type;
        private String assetId;
        private String slideId;
        private String attachmentId;
        private String originalName;
        private String selectionId;
        private Integer markerNo;
        private String html;
        private String htmlHash;
        private String comment;
        private AppendRunDto.DomRegionDto region;
    }
}
