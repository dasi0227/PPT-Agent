package com.dasi.ppt.agent.domain.agent.model.enumeration;

import com.fasterxml.jackson.annotation.JsonProperty;

public enum PlanDecision {
    @JsonProperty("approve")
    APPROVE,
    @JsonProperty("refuse")
    REFUSE
}
