package com.dasi.ppt.agent.domain.setting.model;

public record RouteSettings(
        String mainDefault,
        String mainFallback,
        String sideDefault,
        String sideFallback,
        String rename,
        String compact,
        String commit,
        String polish,
        String handoff,
        String suggest
) {
}
