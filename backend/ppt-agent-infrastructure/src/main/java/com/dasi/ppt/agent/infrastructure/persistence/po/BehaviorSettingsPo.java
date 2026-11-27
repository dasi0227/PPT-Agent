package com.dasi.ppt.agent.infrastructure.persistence.po;

import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Getter;
import lombok.NoArgsConstructor;
import lombok.Setter;

@Builder
@Getter
@Setter
@NoArgsConstructor
@AllArgsConstructor
public class BehaviorSettingsPo {

    private int id;
    private Boolean autoRenameEnabled;
    private Boolean autoSuggestEnabled;
    private Boolean showToolFailures;
    private Boolean requireResourceApproval;
    private Boolean requireHighRiskApproval;
}
