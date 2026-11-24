package com.dasi.ppt.agent.domain.agent.model.enumeration;

import com.dasi.ppt.agent.domain.shared.exception.InvalidEnumValueException;
import com.fasterxml.jackson.annotation.JsonCreator;
import com.fasterxml.jackson.annotation.JsonValue;
import lombok.AllArgsConstructor;
import lombok.Getter;

@Getter
@AllArgsConstructor
public enum ToolResultStatus {
    COMPLETED("completed"),
    BLOCKED("blocked"),
    FAILED("failed");

    @JsonValue
    private final String value;

    @JsonCreator
    public static ToolResultStatus fromValue(String value) {
        for (ToolResultStatus candidate : values()) {
            if (candidate.value.equals(value)) {
                return candidate;
            }
        }
        throw new InvalidEnumValueException(ToolResultStatus.class, value);
    }
}
