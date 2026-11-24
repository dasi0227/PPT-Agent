package com.dasi.ppt.agent.infrastructure.persistence.filesystem;

import com.dasi.ppt.agent.domain.shared.exception.InvalidEnumValueException;
import com.fasterxml.jackson.annotation.JsonCreator;
import com.fasterxml.jackson.annotation.JsonValue;
import lombok.AllArgsConstructor;
import lombok.Getter;

@Getter
@AllArgsConstructor
public enum MutationLogType {
    AGENT("agent"),
    MANUAL("manual"),
    SWITCH("switch");

    @JsonValue
    private final String value;

    @JsonCreator
    public static MutationLogType fromValue(String value) {
        for (MutationLogType candidate : values()) {
            if (candidate.value.equals(value)) {
                return candidate;
            }
        }
        throw new InvalidEnumValueException(MutationLogType.class, value);
    }
}
