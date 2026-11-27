package com.dasi.ppt.agent.domain.agent.model.enumeration;

import com.dasi.ppt.agent.domain.shared.exception.InvalidEnumValueException;
import com.fasterxml.jackson.annotation.JsonCreator;
import com.fasterxml.jackson.annotation.JsonValue;
import lombok.AllArgsConstructor;
import lombok.Getter;

@Getter
@AllArgsConstructor
public enum HarnessErrorCode {
    RUNTIME_BUDGET_EXCEEDED("RUNTIME_BUDGET_EXCEEDED"),
    CONSECUTIVE_TOOL_ERRORS("CONSECUTIVE_TOOL_ERRORS"),
    REPEATED_READ_NO_PROGRESS("REPEATED_READ_NO_PROGRESS"),
    COMPLETION_BLOCK_REPEAT("COMPLETION_BLOCK_REPEAT");

    @JsonValue
    private final String value;

    @JsonCreator
    public static HarnessErrorCode fromValue(String value) {
        for (HarnessErrorCode candidate : values()) {
            if (candidate.value.equals(value)) {
                return candidate;
            }
        }
        throw new InvalidEnumValueException(HarnessErrorCode.class, value);
    }
}
