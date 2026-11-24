package com.dasi.ppt.agent.domain.agent.model.enumeration;

import com.dasi.ppt.agent.domain.shared.exception.InvalidEnumValueException;
import com.fasterxml.jackson.annotation.JsonCreator;
import com.fasterxml.jackson.annotation.JsonValue;
import lombok.AllArgsConstructor;
import lombok.Getter;

@Getter
@AllArgsConstructor
public enum RunMode {
    CHAT("chat"),
    ASK("ask"),
    PLAN("plan"),
    CREATE("create");

    @JsonValue
    private final String value;

    @JsonCreator
    public static RunMode fromValue(String value) {
        for (RunMode candidate : values()) {
            if (candidate.value.equals(value)) {
                return candidate;
            }
        }
        throw new InvalidEnumValueException(RunMode.class, value);
    }
}
