package com.dasi.ppt.agent.domain.agent.model.enumeration;

import com.dasi.ppt.agent.domain.shared.exception.InvalidEnumValueException;
import com.fasterxml.jackson.annotation.JsonCreator;
import com.fasterxml.jackson.annotation.JsonValue;
import lombok.AllArgsConstructor;
import lombok.Getter;

@Getter
@AllArgsConstructor
public enum InstructionReferenceType {
    SKILL("skill"),
    COMPONENT("component"),
    SLIDE("slide"),
    IMAGE("image"),
    DOM("dom");

    @JsonValue
    private final String value;

    @JsonCreator
    public static InstructionReferenceType fromValue(String value) {
        for (InstructionReferenceType candidate : values()) {
            if (candidate.value.equals(value)) {
                return candidate;
            }
        }
        throw new InvalidEnumValueException(InstructionReferenceType.class, value);
    }
}
