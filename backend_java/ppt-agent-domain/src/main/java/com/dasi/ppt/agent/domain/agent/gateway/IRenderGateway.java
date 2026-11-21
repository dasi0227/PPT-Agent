package com.dasi.ppt.agent.domain.agent.gateway;

import com.dasi.ppt.agent.domain.project.model.enumeration.DecorationPosition;
import com.dasi.ppt.agent.domain.project.model.enumeration.DecorationType;

import java.util.List;
import java.util.Map;
import java.util.function.BooleanSupplier;

public interface IRenderGateway {

    Result render(Input input, BooleanSupplier cancelled);

    record Input(
            String slideId,
            String html,
            String themeCss,
            Map<DecorationType, DecorationPosition> positions,
            String deckTitle,
            String sectionTitle,
            int pageNumber,
            byte[] logo
    ) {
    }

    record Result(byte[] screenshot, List<String> layoutIssues) {
    }
}
