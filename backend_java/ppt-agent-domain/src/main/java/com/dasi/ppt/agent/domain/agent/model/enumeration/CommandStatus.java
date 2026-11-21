package com.dasi.ppt.agent.domain.agent.model.enumeration;

import com.fasterxml.jackson.annotation.JsonProperty;

public enum CommandStatus {
    @JsonProperty("pending")
    PENDING,
    @JsonProperty("running")
    RUNNING,
    @JsonProperty("cancelling")
    CANCELLING,
    @JsonProperty("done")
    DONE,
    @JsonProperty("failed")
    FAILED,
    @JsonProperty("interrupted")
    INTERRUPTED,
    @JsonProperty("cancelled")
    CANCELLED
}
