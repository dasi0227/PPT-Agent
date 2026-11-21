package com.dasi.ppt.agent.domain.agent.model.enumeration;

import com.fasterxml.jackson.annotation.JsonProperty;

public enum ResourceDecision {
    @JsonProperty("approve")
    APPROVE,
    @JsonProperty("reject")
    REJECT
}
