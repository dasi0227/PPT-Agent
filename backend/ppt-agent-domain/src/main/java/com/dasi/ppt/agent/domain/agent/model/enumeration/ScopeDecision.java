package com.dasi.ppt.agent.domain.agent.model.enumeration;

import com.dasi.ppt.agent.domain.shared.exception.InvalidEnumValueException;
import com.fasterxml.jackson.annotation.JsonCreator;
import com.fasterxml.jackson.annotation.JsonValue;
import lombok.AllArgsConstructor;
import lombok.Getter;

@Getter
@AllArgsConstructor
public enum ScopeDecision {
    APPROVE("approve"),
    REFUSE("refuse"),
    REVISE("revise");

    @JsonValue
    private final String value;

    @JsonCreator
    public static ScopeDecision fromValue(String value) {
        for (ScopeDecision candidate : values()) {
            if (candidate.value.equals(value)) {
                return candidate;
            }
        }
        throw new InvalidEnumValueException(ScopeDecision.class, value);
    }
}
