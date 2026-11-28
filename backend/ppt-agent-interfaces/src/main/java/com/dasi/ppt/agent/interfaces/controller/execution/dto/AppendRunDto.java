package com.dasi.ppt.agent.interfaces.controller.execution.dto;

import com.dasi.ppt.agent.domain.agent.model.enumeration.InstructionReferenceType;
import com.dasi.ppt.agent.domain.agent.model.enumeration.RunMode;
import com.dasi.ppt.agent.domain.project.model.enumeration.DecorationPosition;
import com.dasi.ppt.agent.domain.project.model.enumeration.DecorationType;
import com.fasterxml.jackson.annotation.JsonCreator;
import com.fasterxml.jackson.annotation.JsonIgnore;
import com.fasterxml.jackson.annotation.JsonSetter;
import lombok.Getter;
import lombok.NoArgsConstructor;
import lombok.Setter;

import java.util.List;

@Getter
@Setter
@NoArgsConstructor(onConstructor_ = @JsonCreator)
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

    @Getter
    public static final class RunScopeDto {
        private String currentSlideId;
        private List<String> slideIds;
        private List<String> chapterIds;
        private List<String> sectionIds;
    }

    @Getter
    public static final class InstructionReferenceDto {
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
        private DomRegionDto region;
    }

    @Getter
    public static final class DomRegionDto {
        private String type;
        private CanvasRectDto rect;
        private List<DomTargetDto> targets;
        private List<DomDecorationDto> decorations;
    }

    @Getter
    public static final class DomTargetDto {
        private String targetId;
        private String tag;
        private DomFingerprintDto fingerprint;
        private CanvasRectDto rect;
    }

    @Getter
    public static final class DomDecorationDto {
        private DecorationType type;
        private DecorationPosition placement;
        private String text;
        private CanvasRectDto rect;
    }

    @Getter
    public static final class DomFingerprintDto {
        private String tag;
        private int siblingIndex;
        private String stableId;
        private List<String> classes;
        private List<DomAncestorDto> ancestors;
    }

    @Getter
    public static final class DomAncestorDto {
        private String tag;
        private String stableId;
        private String classSummary;
        private int siblingIndex;
    }

    @Getter
    public static final class CanvasRectDto {
        private double x;
        private double y;
        private double width;
        private double height;
    }
}
