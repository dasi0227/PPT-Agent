package com.dasi.ppt.agent.domain.setting.model.enumeration;

import com.fasterxml.jackson.annotation.JsonProperty;

public enum LlmProtocol {
    @JsonProperty("responses")
    RESPONSES,
    @JsonProperty("anthropic")
    ANTHROPIC
}
