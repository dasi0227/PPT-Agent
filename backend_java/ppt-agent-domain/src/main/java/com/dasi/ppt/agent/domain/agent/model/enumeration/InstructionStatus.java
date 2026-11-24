package com.dasi.ppt.agent.domain.agent.model.enumeration;

import com.dasi.ppt.agent.domain.shared.exception.InvalidEnumValueException;
import com.fasterxml.jackson.annotation.JsonCreator;
import com.fasterxml.jackson.annotation.JsonValue;
import lombok.AllArgsConstructor;
import lombok.Getter;

@Getter
@AllArgsConstructor
public enum InstructionStatus {
    QUEUED("queued"),
    SENT("sent");

    @JsonValue
    private final String value;

    @JsonCreator
    public static InstructionStatus fromValue(String value) {
        for (InstructionStatus candidate : values()) {
            if (candidate.value.equals(value)) {
                return candidate;
            }
        }
        throw new InvalidEnumValueException(InstructionStatus.class, value);
    }
}
