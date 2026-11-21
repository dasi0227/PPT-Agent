package com.dasi.ppt.agent.domain.project.model.enumeration;

import com.fasterxml.jackson.annotation.JsonProperty;

public enum ChangePart {
    @JsonProperty("meta")
    META,
    @JsonProperty("constraint")
    CONSTRAINT,
    @JsonProperty("outline")
    OUTLINE,
    @JsonProperty("spec")
    SPEC,
    @JsonProperty("html")
    HTML
}
