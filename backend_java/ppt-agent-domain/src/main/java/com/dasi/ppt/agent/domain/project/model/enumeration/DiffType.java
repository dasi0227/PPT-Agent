package com.dasi.ppt.agent.domain.project.model.enumeration;

import com.fasterxml.jackson.annotation.JsonProperty;

public enum DiffType {
    @JsonProperty("fields")
    FIELDS,
    @JsonProperty("outline")
    OUTLINE,
    @JsonProperty("text")
    TEXT,
    @JsonProperty("binary")
    BINARY,
    @JsonProperty("unavailable")
    UNAVAILABLE
}
