package com.dasi.ppt.agent.interfaces.controller.project.dto;

import com.dasi.ppt.agent.domain.project.model.entity.Project;
import com.dasi.ppt.agent.domain.project.model.enumeration.DecorationPosition;
import jakarta.validation.constraints.NotBlank;
import jakarta.validation.constraints.NotNull;
import lombok.Getter;

@Getter
public final class SaveProjectPositionsDto {
    @NotBlank
    private String projectId;
    @NotNull
    private DecorationPosition slidePagination;
    @NotNull
    private DecorationPosition deckTitle;
    @NotNull
    private DecorationPosition chapterTitle;
    @NotNull
    private DecorationPosition logo;

    public Project toEntity() {
        return Project.builder()
                .id(projectId)
                .slidePaginationPosition(slidePagination)
                .deckTitlePosition(deckTitle)
                .chapterTitlePosition(chapterTitle)
                .logoPosition(logo)
                .build();
    }
}
