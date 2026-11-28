package com.dasi.ppt.agent.domain.agent.service.tool.result;

import com.dasi.ppt.agent.domain.agent.model.enumeration.ResourceDecision;
import com.dasi.ppt.agent.domain.agent.service.tool.ToolResult;
import com.dasi.ppt.agent.domain.project.model.entity.DeckChange;
import com.dasi.ppt.agent.domain.project.model.entity.Outline;
import com.fasterxml.jackson.annotation.JsonGetter;
import com.fasterxml.jackson.annotation.JsonIgnore;
import lombok.AccessLevel;
import lombok.Builder;
import lombok.Getter;
import lombok.experimental.SuperBuilder;
import tools.jackson.databind.JsonNode;
import tools.jackson.databind.ObjectMapper;

import java.nio.charset.StandardCharsets;
import java.util.List;

@Getter
@SuperBuilder
public final class EditOutlineResult extends ToolResult {

    @Getter(AccessLevel.NONE)
    private final SavedOutline savedOutline;
    private final List<DeckChange> changes;
    private final JsonNode request;
    private final ResourceDecision decision;
    private final String feedback;

    @JsonIgnore
    public Outline getContent() {
        return savedOutline == null ? null : savedOutline.outline;
    }

    @JsonGetter("content")
    public String getSavedSource() {
        return savedOutline == null ? null : savedOutline.source;
    }

    public static final class SavedOutline {

        private final Outline outline;
        private final String source;

        @Builder
        private SavedOutline(byte[] committedBytes, ObjectMapper objectMapper) {
            this.outline = objectMapper.readValue(committedBytes, Outline.class);
            this.source = new String(committedBytes, StandardCharsets.UTF_8);
        }
    }
}
