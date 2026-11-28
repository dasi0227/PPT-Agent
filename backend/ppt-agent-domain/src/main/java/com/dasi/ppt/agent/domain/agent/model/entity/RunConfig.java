package com.dasi.ppt.agent.domain.agent.model.entity;

import com.dasi.ppt.agent.domain.agent.service.action.ActionType;
import com.dasi.ppt.agent.domain.setting.model.entity.ModelConfigSettings;
import lombok.Builder;
import lombok.Getter;
import lombok.NoArgsConstructor;

import java.util.Map;

@Builder
@Getter
@NoArgsConstructor
public final class RunConfig {
    private ModelConfigSettings main;
    private ModelConfigSettings mainFallback;
    private ModelConfigSettings sideDefault;
    private ModelConfigSettings sideFallback;
    @Builder.Default
    private Map<ActionType, ModelConfigSettings> actionRoutes = Map.of();
    private boolean requireResourceApproval;
    private boolean requireHighRiskApproval;
    private boolean autoRenameEnabled;
    private boolean autoSuggestEnabled;

    public RunConfig(
            ModelConfigSettings main,
            ModelConfigSettings mainFallback,
            ModelConfigSettings sideDefault,
            ModelConfigSettings sideFallback,
            Map<ActionType, ModelConfigSettings> actionRoutes,
            boolean requireResourceApproval,
            boolean requireHighRiskApproval,
            boolean autoRenameEnabled,
            boolean autoSuggestEnabled
    ) {
        this.main = main;
        this.mainFallback = mainFallback;
        this.sideDefault = sideDefault;
        this.sideFallback = sideFallback;
        this.actionRoutes = Map.copyOf(actionRoutes);
        this.requireResourceApproval = requireResourceApproval;
        this.requireHighRiskApproval = requireHighRiskApproval;
        this.autoRenameEnabled = autoRenameEnabled;
        this.autoSuggestEnabled = autoSuggestEnabled;
    }
}
