package com.dasi.ppt.agent.infrastructure.persistence.filesystem;

import com.fasterxml.jackson.annotation.JsonProperty;

public enum MutationLogType {
    @JsonProperty("agent")
    AGENT,
    @JsonProperty("manual")
    MANUAL,
    @JsonProperty("switch")
    SWITCH
}
