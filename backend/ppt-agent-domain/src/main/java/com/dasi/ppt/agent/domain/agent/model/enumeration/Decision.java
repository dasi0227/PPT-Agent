package com.dasi.ppt.agent.domain.agent.model.enumeration;

import com.dasi.ppt.agent.domain.shared.exception.InvalidEnumValueException;
import com.fasterxml.jackson.annotation.JsonCreator;
import com.fasterxml.jackson.annotation.JsonValue;
import lombok.AllArgsConstructor;
import lombok.Getter;

@Getter
@AllArgsConstructor
public enum Decision {
    APPROVE("approve"),
    REFUSE("refuse");

    @JsonValue
    private final String value;

    @JsonCreator
    public static Decision fromValue(String value) {
        for (Decision candidate : values()) {
            if (candidate.value.equals(value)) {
                return candidate;
            }
        }
        throw new InvalidEnumValueException(Decision.class, value);
    }
}
