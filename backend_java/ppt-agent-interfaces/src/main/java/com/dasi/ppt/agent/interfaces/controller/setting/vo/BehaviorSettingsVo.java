package com.dasi.ppt.agent.interfaces.controller.setting.vo;

public record BehaviorSettingsVo(
        boolean autoRenameEnabled,
        boolean autoSuggestEnabled,
        boolean showToolFailures,
        boolean requireResourceApproval,
        boolean requireHighRiskApproval
) {
}
