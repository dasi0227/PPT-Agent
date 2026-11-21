package com.dasi.ppt.agent.interfaces.controller.setting.dto;

import java.util.List;

public record SaveFileOpenSettingsDto(
        String defaultOpen,
        String jsonOpen,
        String htmlOpen,
        List<String> customApps
) {
}
