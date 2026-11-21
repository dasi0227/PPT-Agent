package com.dasi.ppt.agent.domain.agent.model.enumeration;

import com.fasterxml.jackson.annotation.JsonProperty;

public enum InstructionReferenceType {
    @JsonProperty("skill")
    SKILL,
    @JsonProperty("component")
    COMPONENT,
    @JsonProperty("slide")
    SLIDE,
    @JsonProperty("image")
    IMAGE,
    @JsonProperty("dom")
    DOM
}
