package com.dasi.ppt.agent.interfaces.controller.checkpoint.vo;

import com.dasi.ppt.agent.domain.agent.model.enumeration.InstructionReferenceType;
import com.dasi.ppt.agent.domain.agent.model.enumeration.RunMode;
import com.dasi.ppt.agent.interfaces.controller.execution.dto.AppendRunDto;

import java.util.List;

public record RollbackVo(String revision, RollbackInputVo input) {

    public record RollbackInputVo(
            String text,
            List<FullReferenceVo> references,
            RunMode mode,
            String model,
            InitialScopeVo scope
    ) {
    }

    public record InitialScopeVo(
            String currentSlideId,
            List<String> slideIds,
            List<String> sectionIds,
            List<String> subsectionIds
    ) {
    }

    public record FullReferenceVo(
            InstructionReferenceType type,
            String assetId,
            String slideId,
            String attachmentId,
            String originalName,
            String selectionId,
            Integer markerNo,
            String html,
            String htmlHash,
            String comment,
            AppendRunDto.DomRegionDto region
    ) {
    }
}
