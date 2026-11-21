package com.dasi.ppt.agent.domain.setting.model;

import java.util.List;

public record FileOpenSettings(
        String defaultOpen,
        String jsonOpen,
        String htmlOpen,
        List<String> customApps
) {
}
