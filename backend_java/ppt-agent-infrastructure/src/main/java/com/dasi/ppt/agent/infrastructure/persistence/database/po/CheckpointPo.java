package com.dasi.ppt.agent.infrastructure.persistence.database.po;

import lombok.Getter;
import lombok.Setter;

@Getter
@Setter
public class CheckpointPo {

    private String sessionId;
    private String runId;
    private String snapshot;
}
