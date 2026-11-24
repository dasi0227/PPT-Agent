package com.dasi.ppt.agent.interfaces.controller.project.dto;

import com.dasi.ppt.agent.domain.project.model.entity.Project;
import com.dasi.ppt.agent.domain.project.model.enumeration.DecorationPosition;
import jakarta.validation.constraints.NotBlank;
import jakarta.validation.constraints.NotNull;
import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Getter;
import lombok.NoArgsConstructor;

@Builder
@Getter
@AllArgsConstructor
@NoArgsConstructor
public final class SaveProjectPositionsDto {
    @NotBlank
    private String projectId;
    @NotNull
    private DecorationPosition slidePaginationPosition;
    @NotNull
    private DecorationPosition deckTitlePosition;
    @NotNull
    private DecorationPosition sectionTitlePosition;
    @NotNull
    private DecorationPosition logoPosition;

    public Project toProject() {
        return Project.builder()
                .id(projectId)
                .slidePaginationPosition(slidePaginationPosition)
                .deckTitlePosition(deckTitlePosition)
                .sectionTitlePosition(sectionTitlePosition)
                .logoPosition(logoPosition)
                .build();
    }
}
