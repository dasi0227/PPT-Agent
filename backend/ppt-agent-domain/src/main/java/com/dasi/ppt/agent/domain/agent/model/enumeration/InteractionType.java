package com.dasi.ppt.agent.domain.agent.model.enumeration;

import com.dasi.ppt.agent.domain.shared.exception.InvalidEnumValueException;
import com.fasterxml.jackson.annotation.JsonCreator;
import com.fasterxml.jackson.annotation.JsonValue;
import lombok.AllArgsConstructor;
import lombok.Getter;

@Getter
@AllArgsConstructor
public enum InteractionType {
    QUESTION("question"),
    PLAN("plan"),
    PERMISSION("permission"),
    SCOPE("scope"),
    RESOURCE("resource");

    @JsonValue
    private final String value;

    @JsonCreator
    public static InteractionType fromValue(String value) {
        for (InteractionType candidate : values()) {
            if (candidate.value.equals(value)) {
                return candidate;
            }
        }
        throw new InvalidEnumValueException(InteractionType.class, value);
    }
}
