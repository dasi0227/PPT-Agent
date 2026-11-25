package com.dasi.ppt.agent.domain.setting.model.entity;

import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Data;
import lombok.NoArgsConstructor;

@Builder
@Data
@AllArgsConstructor
@NoArgsConstructor
public final class ShortcutSettings {
    private String commandTrigger;
    private String referenceTrigger;
    private String componentTrigger;
    private String slideTrigger;
    private String snippetTrigger;
    private String previousSlide;
    private String nextSlide;
    private String zoomIn;
    private String zoomOut;
    private String toggleOverview;
    private String exportDeck;
    private String toggleView;
    private String toggleSource;
    private String switchTheme;
    private String submitRequest;
    private String selectSuggestion;
}
