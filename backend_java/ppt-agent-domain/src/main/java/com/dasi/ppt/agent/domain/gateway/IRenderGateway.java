package com.dasi.ppt.agent.domain.gateway;

import com.dasi.ppt.agent.domain.project.model.enumeration.DecorationPosition;
import com.dasi.ppt.agent.domain.project.model.enumeration.DecorationType;
import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Getter;
import lombok.NoArgsConstructor;

import java.util.List;
import java.util.Map;
import java.util.function.BooleanSupplier;

public interface IRenderGateway {

    Result render(Input input, BooleanSupplier cancelled);

    @Builder
    @Getter
    @AllArgsConstructor
    @NoArgsConstructor
    final class Input {
        private String slideId;
        private String html;
        private String themeCss;
        private Map<DecorationType, DecorationPosition> positions;
        private String deckTitle;
        private String sectionTitle;
        private int pageNumber;
        private byte[] logo;
    }

    @Builder
    @Getter
    @AllArgsConstructor
    @NoArgsConstructor
    final class Result {
        private byte[] screenshot;
        private List<String> layoutIssues;
    }
}
