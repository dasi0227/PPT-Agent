package com.dasi.ppt.agent.domain.agent.model.enumeration;

import com.fasterxml.jackson.annotation.JsonProperty;

public enum RunStatus {
    @JsonProperty("pending")
    PENDING,
    @JsonProperty("running")
    RUNNING,
    @JsonProperty("waiting")
    WAITING,
    @JsonProperty("cancelling")
    CANCELLING,
    @JsonProperty("done")
    DONE,
    @JsonProperty("rejected")
    REJECTED,
    @JsonProperty("failed")
    FAILED,
    @JsonProperty("interrupted")
    INTERRUPTED,
    @JsonProperty("cancelled")
    CANCELLED
}
