package com.dasi.ppt.agent.interfaces.controller.project.dto;

import com.dasi.ppt.agent.domain.project.model.enumeration.DecorationPosition;
import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Getter;
import lombok.NoArgsConstructor;

@Builder
@Getter
@AllArgsConstructor
@NoArgsConstructor
public final class SaveProjectAppearanceDto {
    private String projectId;
    private String themeId;
    private DecorationPositionsDto positions;

    @Builder
    @Getter
    @AllArgsConstructor
    @NoArgsConstructor
    public static final class DecorationPositionsDto {
        private DecorationPosition slidePagination;
        private DecorationPosition deckTitle;
        private DecorationPosition sectionTitle;
        private DecorationPosition logo;
    }
}
