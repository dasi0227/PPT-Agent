package com.dasi.ppt.agent.domain.project.model.enumeration;

import com.fasterxml.jackson.annotation.JsonProperty;

public enum SlidePurpose {
    @JsonProperty("cover")
    COVER,
    @JsonProperty("introduction")
    INTRODUCTION,
    @JsonProperty("transition")
    TRANSITION,
    @JsonProperty("content_explanation")
    CONTENT_EXPLANATION,
    @JsonProperty("content_comparison")
    CONTENT_COMPARISON,
    @JsonProperty("content_example")
    CONTENT_EXAMPLE,
    @JsonProperty("content_guidance")
    CONTENT_GUIDANCE,
    @JsonProperty("content_other")
    CONTENT_OTHER,
    @JsonProperty("conclusion")
    CONCLUSION,
    @JsonProperty("other")
    OTHER
}
