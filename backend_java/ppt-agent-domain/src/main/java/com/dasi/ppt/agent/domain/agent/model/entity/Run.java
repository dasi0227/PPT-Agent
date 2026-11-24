package com.dasi.ppt.agent.domain.agent.model.entity;

import com.dasi.ppt.agent.domain.agent.model.RunScope;
import com.dasi.ppt.agent.domain.agent.model.enumeration.RunMode;
import com.dasi.ppt.agent.domain.agent.model.enumeration.RunStatus;
import com.dasi.ppt.agent.domain.project.model.entity.DeckChange;
import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Data;
import lombok.NoArgsConstructor;
import tools.jackson.databind.JsonNode;

import java.util.List;

@Builder
@Data
@AllArgsConstructor
@NoArgsConstructor
public final class Run {
    private String id;
    private String sessionId;
    private RunMode mode;
    private String model;
    private RunScope scope;
    private RunStatus status;
    private List<DeckChange> changes;
    private JsonNode error;
    private long inputTokens;
    private long outputTokens;
    private long totalTokens;
    private long callCount;
    private long activeDuration;
    private Long activeSince;
    private long createdAt;
    private long updatedAt;
}
