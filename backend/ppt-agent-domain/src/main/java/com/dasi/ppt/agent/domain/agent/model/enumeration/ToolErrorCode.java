package com.dasi.ppt.agent.domain.agent.model.enumeration;

import com.dasi.ppt.agent.domain.shared.exception.InvalidEnumValueException;
import com.fasterxml.jackson.annotation.JsonCreator;
import com.fasterxml.jackson.annotation.JsonValue;
import lombok.AllArgsConstructor;
import lombok.Getter;

@Getter
@AllArgsConstructor
public enum ToolErrorCode {
    TOOL_ARGUMENT_INVALID("TOOL_ARGUMENT_INVALID"),
    TOOL_CALL_INVALID("TOOL_CALL_INVALID"),
    EDIT_AMBIGUOUS("EDIT_AMBIGUOUS"),
    EDIT_NOT_FOUND("EDIT_NOT_FOUND"),
    OUT_OF_SCOPE("OUT_OF_SCOPE"),
    RENDER_FAILED("RENDER_FAILED");

    @JsonValue
    private final String value;

    @JsonCreator
    public static ToolErrorCode fromValue(String value) {
        for (ToolErrorCode candidate : values()) {
            if (candidate.value.equals(value)) {
                return candidate;
            }
        }
        throw new InvalidEnumValueException(ToolErrorCode.class, value);
    }
}
