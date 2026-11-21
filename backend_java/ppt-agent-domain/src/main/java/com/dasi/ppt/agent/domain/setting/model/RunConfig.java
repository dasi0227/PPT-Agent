package com.dasi.ppt.agent.domain.setting.model;

import com.dasi.ppt.agent.domain.agent.model.enumeration.CommandType;
import com.dasi.ppt.agent.domain.setting.model.entity.LlmConfig;

import java.util.Map;

public record RunConfig(
        LlmConfig main,
        LlmConfig mainFallback,
        LlmConfig sideDefault,
        LlmConfig sideFallback,
        Map<CommandType, LlmConfig> commandRoutes,
        boolean requireResourceApproval,
        boolean requireHighRiskApproval,
        boolean autoRenameEnabled,
        boolean autoSuggestEnabled
) {
    public RunConfig {
        commandRoutes = Map.copyOf(commandRoutes);
    }
}
