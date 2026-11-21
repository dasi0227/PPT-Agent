package com.dasi.ppt.agent.domain.project.model.enumeration;

import com.fasterxml.jackson.annotation.JsonProperty;

public enum DecorationType {
    @JsonProperty("slide_pagination")
    SLIDE_PAGINATION,
    @JsonProperty("deck_title")
    DECK_TITLE,
    @JsonProperty("section_title")
    SECTION_TITLE,
    @JsonProperty("logo")
    LOGO
}
