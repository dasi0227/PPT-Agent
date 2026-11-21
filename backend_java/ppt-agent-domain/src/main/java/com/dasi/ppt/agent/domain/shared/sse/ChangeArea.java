package com.dasi.ppt.agent.domain.shared.sse;

import com.fasterxml.jackson.annotation.JsonProperty;

public enum ChangeArea {
    @JsonProperty("history")
    HISTORY,
    @JsonProperty("run")
    RUN,
    @JsonProperty("commands")
    COMMANDS,
    @JsonProperty("plan")
    PLAN,
    @JsonProperty("suggestions")
    SUGGESTIONS,
    @JsonProperty("context")
    CONTEXT,
    @JsonProperty("checkpoints")
    CHECKPOINTS,
    @JsonProperty("content")
    CONTENT,
    @JsonProperty("sessions")
    SESSIONS
}
