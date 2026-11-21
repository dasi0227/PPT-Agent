package com.dasi.ppt.agent.domain.agent.model.enumeration;

import com.fasterxml.jackson.annotation.JsonProperty;

public enum MessageType {
    @JsonProperty("instruction")
    INSTRUCTION,
    @JsonProperty("response")
    RESPONSE,
    @JsonProperty("thinking")
    THINKING,
    @JsonProperty("commentary")
    COMMENTARY,
    @JsonProperty("tool_call")
    TOOL_CALL,
    @JsonProperty("tool_result")
    TOOL_RESULT,
    @JsonProperty("summary")
    SUMMARY
}
