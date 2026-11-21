package com.dasi.ppt.agent.domain.project.model;

import com.dasi.ppt.agent.domain.project.model.enumeration.DecorationPosition;
import com.dasi.ppt.agent.domain.project.model.enumeration.DecorationType;

import java.util.List;
import java.util.Map;

public record ExportSnapshot(
        String projectName,
        String deckTitle,
        List<Page> pages,
        String themeCss,
        Map<DecorationType, DecorationPosition> positions,
        byte[] logo
) {

    public record Page(
            String title,
            String html,
            String sectionTitle
    ) {
    }
}
