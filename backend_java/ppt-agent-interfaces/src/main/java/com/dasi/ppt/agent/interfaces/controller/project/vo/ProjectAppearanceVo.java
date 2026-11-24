package com.dasi.ppt.agent.interfaces.controller.project.vo;

import com.dasi.ppt.agent.domain.project.model.entity.Project;
import com.dasi.ppt.agent.domain.project.model.enumeration.DecorationPosition;
import com.dasi.ppt.agent.domain.project.model.enumeration.DecorationType;
import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Getter;
import lombok.NoArgsConstructor;

@Builder
@Getter
@AllArgsConstructor
@NoArgsConstructor
public final class ProjectAppearanceVo {
    private String themeId;
    private DecorationPositionsVo positions;
    private boolean logo;

    public static ProjectAppearanceVo toVo(Project project) {
        return ProjectAppearanceVo.builder()
                .themeId(project.getTheme())
                .positions(DecorationPositionsVo.builder()
                        .slidePagination(project.getPositions().get(DecorationType.SLIDE_PAGINATION))
                        .deckTitle(project.getPositions().get(DecorationType.DECK_TITLE))
                        .sectionTitle(project.getPositions().get(DecorationType.SECTION_TITLE))
                        .logo(project.getPositions().get(DecorationType.LOGO))
                        .build())
                .logo(project.isLogoPresent())
                .build();
    }

    @Builder
    @Getter
    @AllArgsConstructor
    @NoArgsConstructor
    public static final class DecorationPositionsVo {
        private DecorationPosition slidePagination;
        private DecorationPosition deckTitle;
        private DecorationPosition sectionTitle;
        private DecorationPosition logo;
    }
}
