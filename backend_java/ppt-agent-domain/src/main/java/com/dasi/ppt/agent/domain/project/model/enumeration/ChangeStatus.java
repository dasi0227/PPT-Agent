package com.dasi.ppt.agent.domain.project.model.enumeration;

import com.fasterxml.jackson.annotation.JsonProperty;

public enum ChangeStatus {
    @JsonProperty("added")
    ADDED,
    @JsonProperty("modified")
    MODIFIED,
    @JsonProperty("deleted")
    DELETED
}
