package com.dasi.ppt.agent.domain.agent.model.enumeration;

import com.fasterxml.jackson.annotation.JsonProperty;

public enum PlanStepStatus {
    @JsonProperty("pending")
    PENDING,
    @JsonProperty("processing")
    PROCESSING,
    @JsonProperty("completed")
    COMPLETED
}
