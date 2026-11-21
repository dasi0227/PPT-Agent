package com.dasi.ppt.agent.interfaces.controller.project.vo;

import com.dasi.ppt.agent.domain.project.model.enumeration.DecorationPosition;

public record ProjectAppearanceVo(
        String themeId,
        DecorationPositionsVo positions,
        boolean logo
) {

    public record DecorationPositionsVo(
            DecorationPosition slidePagination,
            DecorationPosition deckTitle,
            DecorationPosition sectionTitle,
            DecorationPosition logo
    ) {
    }
}
