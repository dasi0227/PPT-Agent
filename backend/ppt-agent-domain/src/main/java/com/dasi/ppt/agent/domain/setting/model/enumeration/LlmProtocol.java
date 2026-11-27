package com.dasi.ppt.agent.domain.setting.model.enumeration;

import com.dasi.ppt.agent.domain.shared.exception.InvalidEnumValueException;
import com.fasterxml.jackson.annotation.JsonCreator;
import com.fasterxml.jackson.annotation.JsonValue;
import lombok.AllArgsConstructor;
import lombok.Getter;

@Getter
@AllArgsConstructor
public enum LlmProtocol {
    RESPONSES("responses"),
    ANTHROPIC("anthropic");

    @JsonValue
    private final String value;

    @JsonCreator
    public static LlmProtocol fromValue(String value) {
        for (LlmProtocol candidate : values()) {
            if (candidate.value.equals(value)) {
                return candidate;
            }
        }
        throw new InvalidEnumValueException(LlmProtocol.class, value);
    }
}
