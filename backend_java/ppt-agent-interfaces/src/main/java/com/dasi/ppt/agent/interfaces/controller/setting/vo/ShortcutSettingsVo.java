package com.dasi.ppt.agent.interfaces.controller.setting.vo;

import com.dasi.ppt.agent.domain.setting.model.entity.ShortcutSettings;
import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Getter;
import lombok.NoArgsConstructor;

@Builder
@Getter
@AllArgsConstructor
@NoArgsConstructor
public final class ShortcutSettingsVo {
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

    public static ShortcutSettingsVo toVo(ShortcutSettings settings) {
        return ShortcutSettingsVo.builder()
                .commandTrigger(settings.getCommandTrigger())
                .referenceTrigger(settings.getReferenceTrigger())
                .componentTrigger(settings.getComponentTrigger())
                .slideTrigger(settings.getSlideTrigger())
                .snippetTrigger(settings.getSnippetTrigger())
                .previousSlide(settings.getPreviousSlide())
                .nextSlide(settings.getNextSlide())
                .zoomIn(settings.getZoomIn())
                .zoomOut(settings.getZoomOut())
                .toggleOverview(settings.getToggleOverview())
                .exportDeck(settings.getExportDeck())
                .toggleView(settings.getToggleView())
                .toggleSource(settings.getToggleSource())
                .switchTheme(settings.getSwitchTheme())
                .submitRequest(settings.getSubmitRequest())
                .selectSuggestion(settings.getSelectSuggestion())
                .build();
    }
}
