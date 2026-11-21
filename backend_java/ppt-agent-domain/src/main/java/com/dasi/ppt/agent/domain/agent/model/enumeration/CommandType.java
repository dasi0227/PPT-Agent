package com.dasi.ppt.agent.domain.agent.model.enumeration;

import com.fasterxml.jackson.annotation.JsonProperty;

public enum CommandType {
    @JsonProperty("rename")
    RENAME,
    @JsonProperty("polish")
    POLISH,
    @JsonProperty("handoff")
    HANDOFF,
    @JsonProperty("compact")
    COMPACT,
    @JsonProperty("commit")
    COMMIT,
    @JsonProperty("suggest")
    SUGGEST
}
