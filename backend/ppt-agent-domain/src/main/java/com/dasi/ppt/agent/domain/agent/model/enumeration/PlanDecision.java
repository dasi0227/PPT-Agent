package com.dasi.ppt.agent.domain.agent.model.enumeration;

import com.dasi.ppt.agent.domain.shared.exception.InvalidEnumValueException;
import com.fasterxml.jackson.annotation.JsonCreator;
import com.fasterxml.jackson.annotation.JsonValue;
import lombok.AllArgsConstructor;
import lombok.Getter;

@Getter
@AllArgsConstructor
public enum PlanDecision {
    APPROVE("approve"),
    REFUSE("refuse");

    @JsonValue
    private final String value;

    @JsonCreator
    public static PlanDecision fromValue(String value) {
        for (PlanDecision candidate : values()) {
            if (candidate.value.equals(value)) {
                return candidate;
            }
        }
        throw new InvalidEnumValueException(PlanDecision.class, value);
    }
}
