package com.dasi.ppt.agent.interfaces.controller.setting.vo;

import java.util.List;

public record FileOpenSettingsVo(
        String defaultOpen,
        String jsonOpen,
        String htmlOpen,
        List<String> customApps,
        boolean supported
) {
}
