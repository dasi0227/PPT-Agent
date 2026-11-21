package com.dasi.ppt.agent.infrastructure.persistence.database;

import com.fasterxml.jackson.annotation.JsonProperty;

public enum ReceiptType {
    @JsonProperty("create_run")
    CREATE_RUN,
    @JsonProperty("create_command")
    CREATE_COMMAND,
    @JsonProperty("agent")
    AGENT,
    @JsonProperty("manual")
    MANUAL
}
