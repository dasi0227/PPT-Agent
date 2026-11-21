package com.dasi.ppt.agent.domain.setting.model;

public record BehaviorSettings(
        boolean autoRenameEnabled,
        boolean autoSuggestEnabled,
        boolean showToolFailures,
        boolean requireResourceApproval,
        boolean requireHighRiskApproval
) {
}
