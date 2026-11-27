package com.dasi.ppt.agent.domain.agent.model.enumeration;

import com.dasi.ppt.agent.domain.shared.exception.InvalidEnumValueException;
import com.fasterxml.jackson.annotation.JsonCreator;
import com.fasterxml.jackson.annotation.JsonValue;
import lombok.AllArgsConstructor;
import lombok.Getter;

@Getter
@AllArgsConstructor
public enum InstructionDecision {
    TASK("task"),
    GOSSIP("gossip"),
    DISCLOSURE("disclosure"),
    BYPASS("bypass"),
    IRRELEVANT("irrelevant");

    @JsonValue
    private final String value;

    @JsonCreator
    public static InstructionDecision fromValue(String value) {
        for (InstructionDecision candidate : values()) {
            if (candidate.value.equals(value)) {
                return candidate;
            }
        }
        throw new InvalidEnumValueException(InstructionDecision.class, value);
    }
}
