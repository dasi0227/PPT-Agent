package com.dasi.ppt.agent.infrastructure.persistence.database.po;

import lombok.Getter;
import lombok.Setter;

@Getter
@Setter
public class SessionPo {

    private String id;
    private String projectId;
    private long revision;
    private String title;
    private String plan;
    private long updatedAt;
}
