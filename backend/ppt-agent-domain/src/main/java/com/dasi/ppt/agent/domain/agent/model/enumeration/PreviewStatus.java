package com.dasi.ppt.agent.domain.agent.model.enumeration;

import com.dasi.ppt.agent.domain.shared.exception.InvalidEnumValueException;
import com.fasterxml.jackson.annotation.JsonCreator;
import com.fasterxml.jackson.annotation.JsonValue;
import lombok.AllArgsConstructor;
import lombok.Getter;

@Getter
@AllArgsConstructor
public enum PreviewStatus {
    COMPLETED("completed"),
    UNAVAILABLE("unavailable"),
    STALE("stale");

    @JsonValue
    private final String value;

    @JsonCreator
    public static PreviewStatus fromValue(String value) {
        for (PreviewStatus candidate : values()) {
            if (candidate.value.equals(value)) {
                return candidate;
            }
        }
        throw new InvalidEnumValueException(PreviewStatus.class, value);
    }
}
