package com.dasi.ppt.agent.interfaces.controller.project.vo;

import com.dasi.ppt.agent.domain.project.model.entity.Project;
import com.dasi.ppt.agent.domain.project.model.enumeration.DecorationPosition;
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
                        .slidePagination(project.getSlidePaginationPosition())
                        .deckTitle(project.getDeckTitlePosition())
                        .chapterTitle(project.getChapterTitlePosition())
                        .logo(project.getLogoPosition())
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
        private DecorationPosition chapterTitle;
        private DecorationPosition logo;
    }
}
