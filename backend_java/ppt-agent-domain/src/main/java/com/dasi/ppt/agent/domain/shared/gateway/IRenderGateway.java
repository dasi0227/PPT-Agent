package com.dasi.ppt.agent.domain.shared.gateway;

import com.dasi.ppt.agent.domain.project.model.entity.ExportFile;
import com.dasi.ppt.agent.domain.project.model.entity.ExportSnapshot;
import com.dasi.ppt.agent.domain.project.model.enumeration.DecorationPosition;
import com.dasi.ppt.agent.domain.project.model.enumeration.DecorationType;
import com.dasi.ppt.agent.domain.project.model.enumeration.ExportFormat;
import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Getter;
import lombok.NoArgsConstructor;

import java.util.List;
import java.util.Map;
import java.util.function.BooleanSupplier;

public interface IRenderGateway {

    String renderHash(RenderMaterial renderMaterial);

    RenderResult render(RenderMaterial renderMaterial, BooleanSupplier cancelled);

    ExportFile exportDeck(ExportSnapshot snapshot, ExportFormat format);

    @Builder
    @Getter
    @AllArgsConstructor
    @NoArgsConstructor
    final class RenderMaterial {
        private String slideId;
        private String html;
        private String themeCss;
        private Map<DecorationType, DecorationPosition> positions;
        private String deckTitle;
        private String chapterTitle;
        private int pageNumber;
        private int totalPages;
        private byte[] logo;
    }

    @Builder
    @Getter
    @AllArgsConstructor
    @NoArgsConstructor
    final class RenderResult {
        private byte[] screenshot;
        private List<String> layoutIssues;
    }
}
