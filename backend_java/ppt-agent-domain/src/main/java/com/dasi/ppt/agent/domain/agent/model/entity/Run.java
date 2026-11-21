package com.dasi.ppt.agent.domain.agent.model.entity;

import com.dasi.ppt.agent.domain.agent.model.RunScope;
import com.dasi.ppt.agent.domain.agent.model.enumeration.RunMode;
import com.dasi.ppt.agent.domain.agent.model.enumeration.RunStatus;
import com.dasi.ppt.agent.domain.project.model.DeckChange;
import tools.jackson.databind.JsonNode;

import java.util.List;

public record Run(
        String id,
        String sessionId,
        RunMode mode,
        String model,
        RunScope scope,
        RunStatus status,
        List<DeckChange> changes,
        JsonNode error,
        long inputTokens,
        long outputTokens,
        long totalTokens,
        long callCount,
        long activeDuration,
        Long activeSince,
        long createdAt,
        long updatedAt
) {
}
