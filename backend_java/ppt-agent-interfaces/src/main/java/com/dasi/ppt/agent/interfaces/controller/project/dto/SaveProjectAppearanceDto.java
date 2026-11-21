package com.dasi.ppt.agent.interfaces.controller.project.dto;

import com.dasi.ppt.agent.domain.project.model.enumeration.DecorationPosition;

public record SaveProjectAppearanceDto(
        String projectId,
        String themeId,
        DecorationPositionsDto positions
) {

    public record DecorationPositionsDto(
            DecorationPosition slidePagination,
            DecorationPosition deckTitle,
            DecorationPosition sectionTitle,
            DecorationPosition logo
    ) {
    }
}
