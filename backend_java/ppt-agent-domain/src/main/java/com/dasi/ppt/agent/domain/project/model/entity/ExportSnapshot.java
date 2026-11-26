package com.dasi.ppt.agent.domain.project.model.entity;

import com.dasi.ppt.agent.domain.project.model.enumeration.DecorationPosition;
import com.dasi.ppt.agent.domain.project.model.enumeration.DecorationType;
import com.dasi.ppt.agent.domain.shared.gateway.IRenderGateway;
import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Getter;
import lombok.NoArgsConstructor;

import java.util.List;
import java.util.Map;

@Builder
@Getter
@AllArgsConstructor
@NoArgsConstructor
public final class ExportSnapshot {
    private String projectName;
    private String deckTitle;
    private List<Page> pages;
    private String themeCss;
    private Map<DecorationType, DecorationPosition> positions;
    private byte[] logo;

    public IRenderGateway.RenderMaterial renderInput(int index) {
        Page page = pages.get(index);
        return IRenderGateway.RenderMaterial.builder()
                .slideId(page.getSlideId())
                .html(page.getHtml())
                .themeCss(themeCss)
                .positions(positions)
                .deckTitle(deckTitle)
                .chapterTitle(page.getChapterTitle())
                .pageNumber(index + 1)
                .totalPages(pages.size())
                .logo(logo)
                .build();
    }

    @Builder
    @Getter
    @AllArgsConstructor
    @NoArgsConstructor
    public static final class Page {
        private String slideId;
        private String title;
        private String html;
        private String chapterTitle;
    }
}
