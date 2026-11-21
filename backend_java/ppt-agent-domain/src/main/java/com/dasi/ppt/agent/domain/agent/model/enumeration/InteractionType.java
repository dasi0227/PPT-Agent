package com.dasi.ppt.agent.domain.agent.model.enumeration;

import com.fasterxml.jackson.annotation.JsonProperty;

public enum InteractionType {
    @JsonProperty("question")
    QUESTION,
    @JsonProperty("plan")
    PLAN,
    @JsonProperty("permission")
    PERMISSION,
    @JsonProperty("scope")
    SCOPE,
    @JsonProperty("resource")
    RESOURCE
}
