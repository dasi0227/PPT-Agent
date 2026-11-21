package com.dasi.ppt.agent.domain.project.model.enumeration;

import com.fasterxml.jackson.annotation.JsonProperty;

public enum OutlineNodeType {
    @JsonProperty("section")
    SECTION,
    @JsonProperty("subsection")
    SUBSECTION,
    @JsonProperty("slide")
    SLIDE
}
