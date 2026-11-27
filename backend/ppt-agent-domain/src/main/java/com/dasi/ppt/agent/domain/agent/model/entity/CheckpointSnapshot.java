package com.dasi.ppt.agent.domain.agent.model.entity;

import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Getter;
import lombok.NoArgsConstructor;
import tools.jackson.databind.JsonNode;

import java.util.List;
import java.util.Map;

@Builder
@Getter
@AllArgsConstructor
@NoArgsConstructor
public final class CheckpointSnapshot {
    private String title;
    private Plan plan;
    private List<Run> runs;
    private List<Message> messages;
    private List<ActionExecution> actions;
    private Map<String, byte[]> files;
    private JsonNode input;
}
