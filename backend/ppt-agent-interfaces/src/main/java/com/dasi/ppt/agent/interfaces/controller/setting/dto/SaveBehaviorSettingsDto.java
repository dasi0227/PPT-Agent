package com.dasi.ppt.agent.interfaces.controller.setting.dto;

import com.dasi.ppt.agent.domain.setting.model.entity.BehaviorSettings;
import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Getter;
import lombok.NoArgsConstructor;

@Builder
@Getter
@AllArgsConstructor
@NoArgsConstructor
public final class SaveBehaviorSettingsDto {
    private Boolean autoRenameEnabled;
    private Boolean autoSuggestEnabled;
    private Boolean showToolFailures;
    private Boolean requireResourceApproval;
    private Boolean requireHighRiskApproval;

    public BehaviorSettings toEntity() {
        return BehaviorSettings.builder()
                .autoRenameEnabled(autoRenameEnabled)
                .autoSuggestEnabled(autoSuggestEnabled)
                .showToolFailures(showToolFailures)
                .requireResourceApproval(requireResourceApproval)
                .requireHighRiskApproval(requireHighRiskApproval)
                .build();
    }
}
