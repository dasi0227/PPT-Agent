package com.dasi.ppt.agent.domain.agent.model.enumeration;

import com.dasi.ppt.agent.domain.shared.exception.InvalidEnumValueException;
import com.fasterxml.jackson.annotation.JsonCreator;
import com.fasterxml.jackson.annotation.JsonValue;
import lombok.AllArgsConstructor;
import lombok.Getter;

@Getter
@AllArgsConstructor
public enum MessageType {
    INSTRUCTION("instruction"),
    RESPONSE("response"),
    THINKING("thinking"),
    COMMENTARY("commentary"),
    TOOL_CALL("tool_call"),
    TOOL_RESULT("tool_result"),
    SUMMARY("summary");

    @JsonValue
    private final String value;

    @JsonCreator
    public static MessageType fromValue(String value) {
        for (MessageType candidate : values()) {
            if (candidate.value.equals(value)) {
                return candidate;
            }
        }
        throw new InvalidEnumValueException(MessageType.class, value);
    }
}
