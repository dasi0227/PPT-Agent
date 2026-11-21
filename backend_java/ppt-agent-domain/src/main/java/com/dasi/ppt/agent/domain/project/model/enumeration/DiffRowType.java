package com.dasi.ppt.agent.domain.project.model.enumeration;

import com.fasterxml.jackson.annotation.JsonProperty;

public enum DiffRowType {
    @JsonProperty("added")
    ADDED,
    @JsonProperty("removed")
    REMOVED,
    @JsonProperty("context")
    CONTEXT
}
