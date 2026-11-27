package com.dasi.ppt.agent.infrastructure.persistence.po;

import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Getter;
import lombok.NoArgsConstructor;
import lombok.Setter;

@Builder
@Getter
@Setter
@AllArgsConstructor
@NoArgsConstructor
public class SessionPo {

    private String id;
    private String projectId;
    private long revision;
    private String title;
    private String plan;
    private long updatedAt;
    private String runStatus;
    private boolean hasActiveActions;
}
