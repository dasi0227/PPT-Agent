package com.dasi.ppt.agent.interfaces.controller.setting.vo;

import com.dasi.ppt.agent.domain.setting.model.entity.BehaviorSettings;
import lombok.Builder;
import lombok.Getter;

@Builder
@Getter
public final class BehaviorSettingsVo {
    private boolean autoRenameEnabled;
    private boolean autoSuggestEnabled;
    private boolean showToolFailures;
    private boolean requireResourceApproval;
    private boolean requireHighRiskApproval;

    public static BehaviorSettingsVo toVo(BehaviorSettings settings) {
        return BehaviorSettingsVo.builder()
                .autoRenameEnabled(settings.getAutoRenameEnabled())
                .autoSuggestEnabled(settings.getAutoSuggestEnabled())
                .showToolFailures(settings.getShowToolFailures())
                .requireResourceApproval(settings.getRequireResourceApproval())
                .requireHighRiskApproval(settings.getRequireHighRiskApproval())
                .build();
    }
}
