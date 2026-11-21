package com.dasi.ppt.agent.domain.agent.model.enumeration;

import com.fasterxml.jackson.annotation.JsonProperty;

public enum PermissionDecision {
    @JsonProperty("allow_once")
    ALLOW_ONCE,
    @JsonProperty("deny")
    DENY
}
