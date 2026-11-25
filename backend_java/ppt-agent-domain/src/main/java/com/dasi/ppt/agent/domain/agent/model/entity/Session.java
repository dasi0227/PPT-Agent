package com.dasi.ppt.agent.domain.agent.model.entity;

import com.dasi.ppt.agent.domain.agent.model.Plan;
import com.dasi.ppt.agent.domain.agent.model.enumeration.RunStatus;
import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Data;
import lombok.NoArgsConstructor;

@Builder
@Data
@AllArgsConstructor
@NoArgsConstructor
public final class Session {
    private String id;
    private String projectId;
    private long revision;
    private String title;
    private Plan plan;
    private long updatedAt;
    private RunStatus runStatus;
    private boolean hasActiveCommands;
}
