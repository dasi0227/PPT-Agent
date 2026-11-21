package com.dasi.ppt.agent.domain.agent.model.enumeration;

import com.fasterxml.jackson.annotation.JsonProperty;

public enum ScopeDecision {
    @JsonProperty("approve")
    APPROVE,
    @JsonProperty("refuse")
    REFUSE,
    @JsonProperty("revise")
    REVISE
}
