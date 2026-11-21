package com.dasi.ppt.agent.interfaces.controller.execution.dto;

import com.dasi.ppt.agent.domain.agent.model.enumeration.InstructionReferenceType;
import com.dasi.ppt.agent.domain.agent.model.enumeration.RunMode;
import com.dasi.ppt.agent.domain.project.model.enumeration.DecorationPosition;
import com.dasi.ppt.agent.domain.project.model.enumeration.DecorationType;
import com.fasterxml.jackson.annotation.JsonIgnore;
import com.fasterxml.jackson.annotation.JsonSetter;
import lombok.Getter;
import lombok.Setter;

import java.util.List;

@Getter
@Setter
public class AppendRunDto {

    private String sessionId;
    private String runId;
    private String messageId;
    private String text;
    private RunMode mode;
    private String model;
    private RunScopeDto scope;
    private List<InstructionReferenceDto> references;
    private String branchRevision;

    @JsonIgnore
    @Setter(lombok.AccessLevel.NONE)
    private boolean scopeProvided;

    @JsonSetter("scope")
    public void setScope(RunScopeDto scope) {
        this.scopeProvided = true;
        this.scope = scope;
    }

    public record RunScopeDto(
            String currentSlideId,
            List<String> slideIds,
            List<String> sectionIds,
            List<String> subsectionIds
    ) {
    }

    public record InstructionReferenceDto(
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
            DomRegionDto region
    ) {
    }

    public record DomRegionDto(
            String type,
            CanvasRectDto rect,
            List<DomTargetDto> targets,
            List<DomDecorationDto> decorations
    ) {
    }

    public record DomTargetDto(
            String targetId,
            String tag,
            DomFingerprintDto fingerprint,
            CanvasRectDto rect
    ) {
    }

    public record DomDecorationDto(
            DecorationType type,
            DecorationPosition placement,
            String text,
            CanvasRectDto rect
    ) {
    }

    public record DomFingerprintDto(
            String tag,
            int siblingIndex,
            String stableId,
            List<String> classes,
            List<DomAncestorDto> ancestors
    ) {
    }

    public record DomAncestorDto(
            String tag,
            String stableId,
            String classSummary,
            int siblingIndex
    ) {
    }

    public record CanvasRectDto(
            double x,
            double y,
            double width,
            double height
    ) {
    }
}
