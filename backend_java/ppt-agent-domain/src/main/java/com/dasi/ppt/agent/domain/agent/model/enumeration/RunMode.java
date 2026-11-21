package com.dasi.ppt.agent.domain.agent.model.enumeration;

import com.fasterxml.jackson.annotation.JsonProperty;

public enum RunMode {
    @JsonProperty("chat")
    CHAT,
    @JsonProperty("ask")
    ASK,
    @JsonProperty("plan")
    PLAN,
    @JsonProperty("create")
    CREATE
}
