package com.dasi.ppt.agent.domain.project.model.enumeration;

import com.fasterxml.jackson.annotation.JsonProperty;

public enum DecorationPosition {
    @JsonProperty("none")
    NONE,
    @JsonProperty("top-left")
    TOP_LEFT,
    @JsonProperty("top-center")
    TOP_CENTER,
    @JsonProperty("top-right")
    TOP_RIGHT,
    @JsonProperty("bottom-left")
    BOTTOM_LEFT,
    @JsonProperty("bottom-right")
    BOTTOM_RIGHT
}
