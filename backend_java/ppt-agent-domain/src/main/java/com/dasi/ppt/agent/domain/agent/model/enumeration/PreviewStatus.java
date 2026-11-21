package com.dasi.ppt.agent.domain.agent.model.enumeration;

import com.fasterxml.jackson.annotation.JsonProperty;

public enum PreviewStatus {
    @JsonProperty("completed")
    COMPLETED,
    @JsonProperty("unavailable")
    UNAVAILABLE,
    @JsonProperty("stale")
    STALE
}
