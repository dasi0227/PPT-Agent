package com.dasi.ppt.agent.domain.agent.model.enumeration;

import com.fasterxml.jackson.annotation.JsonProperty;

public enum ToolResultStatus {
    @JsonProperty("completed")
    COMPLETED,
    @JsonProperty("blocked")
    BLOCKED,
    @JsonProperty("failed")
    FAILED
}
