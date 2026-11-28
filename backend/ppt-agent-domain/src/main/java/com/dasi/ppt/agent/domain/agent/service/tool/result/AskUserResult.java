package com.dasi.ppt.agent.domain.agent.service.tool.result;

import com.dasi.ppt.agent.domain.agent.service.tool.ToolResult;
import com.fasterxml.jackson.annotation.JsonValue;
import lombok.Builder;
import lombok.Data;
import lombok.Getter;
import lombok.experimental.SuperBuilder;
import tools.jackson.databind.JsonNode;

import java.util.List;
import java.util.Map;

@Getter
@SuperBuilder
public final class AskUserResult extends ToolResult {

    private final List<Answer> answers;
    private final JsonNode request;
    private final JsonNode answer;

    @Data
    @Builder
    public static final class Answer {

        private final String text;

        @JsonValue
        public Object toModelValue() {
            return text == null ? Map.of("state", "skipped") : text;
        }
    }
}
