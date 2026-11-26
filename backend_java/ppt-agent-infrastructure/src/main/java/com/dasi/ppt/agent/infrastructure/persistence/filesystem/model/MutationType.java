package com.dasi.ppt.agent.infrastructure.persistence.filesystem.model;

import com.dasi.ppt.agent.domain.shared.exception.InvalidEnumValueException;
import com.fasterxml.jackson.annotation.JsonCreator;
import com.fasterxml.jackson.annotation.JsonValue;
import lombok.AllArgsConstructor;
import lombok.Getter;

@Getter
@AllArgsConstructor
public enum MutationType {
    AGENT("agent"),
    MANUAL("manual"),
    SWITCH("switch");

    @JsonValue
    private final String value;

    @JsonCreator
    public static MutationType fromValue(String value) {
        for (MutationType candidate : values()) {
            if (candidate.value.equals(value)) {
                return candidate;
            }
        }
        throw new InvalidEnumValueException(MutationType.class, value);
    }
}
