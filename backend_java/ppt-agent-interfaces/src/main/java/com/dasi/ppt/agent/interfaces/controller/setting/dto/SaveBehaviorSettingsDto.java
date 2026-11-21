package com.dasi.ppt.agent.interfaces.controller.setting.dto;

public record SaveBehaviorSettingsDto(
        boolean autoRenameEnabled,
        boolean autoSuggestEnabled,
        boolean showToolFailures,
        boolean requireResourceApproval,
        boolean requireHighRiskApproval
) {
}
