package com.dasi.ppt.agent.interfaces.controller.setting.dto;

import com.fasterxml.jackson.annotation.JsonProperty;

public record SaveShortcutSettingsDto(
        String commandTrigger,
        String referenceTrigger,
        String componentTrigger,
        String slideTrigger,
        String snippetTrigger,
        String previousSlide,
        String nextSlide,
        String zoomIn,
        String zoomOut,
        String toggleOverview,
        String exportDeck,
        String toggleView,
        String toggleSource,
        String switchTheme,
        String submitRequest,
        @JsonProperty("suggestion_1") String suggestion1,
        @JsonProperty("suggestion_2") String suggestion2,
        @JsonProperty("suggestion_3") String suggestion3
) {
}
