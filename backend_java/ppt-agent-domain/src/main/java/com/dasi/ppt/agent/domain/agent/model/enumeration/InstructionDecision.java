package com.dasi.ppt.agent.domain.agent.model.enumeration;

import com.fasterxml.jackson.annotation.JsonProperty;

public enum InstructionDecision {
    @JsonProperty("task")
    TASK,
    @JsonProperty("gossip")
    GOSSIP,
    @JsonProperty("disclosure")
    DISCLOSURE,
    @JsonProperty("bypass")
    BYPASS,
    @JsonProperty("irrelevant")
    IRRELEVANT
}
