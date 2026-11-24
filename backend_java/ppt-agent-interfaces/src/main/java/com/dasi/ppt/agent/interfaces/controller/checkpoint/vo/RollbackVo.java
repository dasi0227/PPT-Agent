package com.dasi.ppt.agent.interfaces.controller.checkpoint.vo;

import com.dasi.ppt.agent.domain.agent.model.enumeration.InstructionReferenceType;
import com.dasi.ppt.agent.domain.agent.model.enumeration.RunMode;
import com.dasi.ppt.agent.interfaces.controller.execution.dto.AppendRunDto;
import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Getter;
import lombok.NoArgsConstructor;

import java.util.List;

@Builder
@Getter
@AllArgsConstructor
@NoArgsConstructor
public final class RollbackVo {
    private String revision;
    private RollbackInputVo input;

    @Builder
    @Getter
    @AllArgsConstructor
    @NoArgsConstructor
    public static final class RollbackInputVo {
        private String text;
        private List<FullReferenceVo> references;
        private RunMode mode;
        private String model;
        private InitialScopeVo scope;
    }

    @Builder
    @Getter
    @AllArgsConstructor
    @NoArgsConstructor
    public static final class InitialScopeVo {
        private String currentSlideId;
        private List<String> slideIds;
        private List<String> sectionIds;
        private List<String> subsectionIds;
    }

    @Builder
    @Getter
    @AllArgsConstructor
    @NoArgsConstructor
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
