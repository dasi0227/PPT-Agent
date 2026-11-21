package com.dasi.ppt.agent.domain.agent.model.enumeration;

import com.fasterxml.jackson.annotation.JsonProperty;

public enum InstructionStatus {
    @JsonProperty("queued")
    QUEUED,
    @JsonProperty("sent")
    SENT
}
