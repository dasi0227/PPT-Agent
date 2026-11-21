package com.dasi.ppt.agent.domain.setting.model;

public record ShortcutSettings(
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
        String suggestion1,
        String suggestion2,
        String suggestion3
) {
}
