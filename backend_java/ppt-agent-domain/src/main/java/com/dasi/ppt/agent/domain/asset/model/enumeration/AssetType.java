package com.dasi.ppt.agent.domain.asset.model.enumeration;

import com.fasterxml.jackson.annotation.JsonProperty;

public enum AssetType {
    @JsonProperty("theme")
    THEME,
    @JsonProperty("component")
    COMPONENT,
    @JsonProperty("skill")
    SKILL,
    @JsonProperty("snippet")
    SNIPPET
}
