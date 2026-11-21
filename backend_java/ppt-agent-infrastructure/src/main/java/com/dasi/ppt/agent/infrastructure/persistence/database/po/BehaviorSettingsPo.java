package com.dasi.ppt.agent.infrastructure.persistence.database.po;

import lombok.Getter;
import lombok.Setter;

@Getter
@Setter
public class BehaviorSettingsPo {

    private int id;
    private boolean autoRenameEnabled;
    private boolean autoSuggestEnabled;
    private boolean showToolFailures;
    private boolean requireResourceApproval;
    private boolean requireHighRiskApproval;
}
