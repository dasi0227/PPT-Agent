package com.dasi.ppt.agent.domain.setting.model.entity;

import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Data;
import lombok.NoArgsConstructor;

@Builder
@Data
@AllArgsConstructor
@NoArgsConstructor
public final class BehaviorSettings {
    private Boolean autoRenameEnabled;
    private Boolean autoSuggestEnabled;
    private Boolean showToolFailures;
    private Boolean requireResourceApproval;
    private Boolean requireHighRiskApproval;

    public boolean hasNoChanges() {
        return autoRenameEnabled == null && autoSuggestEnabled == null && showToolFailures == null
                && requireResourceApproval == null && requireHighRiskApproval == null;
    }
}
