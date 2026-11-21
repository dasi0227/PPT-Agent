package com.dasi.ppt.agent.domain.project.model.enumeration;

import com.fasterxml.jackson.annotation.JsonProperty;

public enum SlideElementType {
    @JsonProperty("text")
    TEXT,
    @JsonProperty("list")
    LIST,
    @JsonProperty("metric")
    METRIC,
    @JsonProperty("quote")
    QUOTE,
    @JsonProperty("table")
    TABLE,
    @JsonProperty("chart")
    CHART,
    @JsonProperty("diagram")
    DIAGRAM,
    @JsonProperty("code")
    CODE,
    @JsonProperty("asset")
    ASSET
}
