package com.dasi.ppt.agent.domain.agent.model.entity;

import com.dasi.ppt.agent.domain.agent.model.enumeration.InteractionType;
import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Data;
import lombok.NoArgsConstructor;
import tools.jackson.databind.JsonNode;

@Builder
@Data
@AllArgsConstructor
@NoArgsConstructor
public final class PendingInteraction {
    private String runId;
    private String toolCallId;
    private InteractionType type;
    private String version;
    private JsonNode request;
}
