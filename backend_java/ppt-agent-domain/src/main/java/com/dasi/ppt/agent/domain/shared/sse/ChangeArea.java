package com.dasi.ppt.agent.domain.shared.sse;

import com.dasi.ppt.agent.domain.shared.exception.InvalidEnumValueException;
import com.fasterxml.jackson.annotation.JsonCreator;
import com.fasterxml.jackson.annotation.JsonValue;
import lombok.AllArgsConstructor;
import lombok.Getter;

@Getter
@AllArgsConstructor
public enum ChangeArea {
    HISTORY("history"),
    RUN("run"),
    COMMANDS("commands"),
    PLAN("plan"),
    SUGGESTIONS("suggestions"),
    CONTEXT("context"),
    CHECKPOINTS("checkpoints"),
    CONTENT("content"),
    SESSIONS("sessions");

    @JsonValue
    private final String value;

    @JsonCreator
    public static ChangeArea fromValue(String value) {
        for (ChangeArea candidate : values()) {
            if (candidate.value.equals(value)) {
                return candidate;
            }
        }
        throw new InvalidEnumValueException(ChangeArea.class, value);
    }
}
