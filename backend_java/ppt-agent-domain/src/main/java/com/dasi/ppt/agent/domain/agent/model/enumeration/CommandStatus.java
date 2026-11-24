package com.dasi.ppt.agent.domain.agent.model.enumeration;

import com.dasi.ppt.agent.domain.shared.exception.InvalidEnumValueException;
import com.fasterxml.jackson.annotation.JsonCreator;
import com.fasterxml.jackson.annotation.JsonValue;
import lombok.AllArgsConstructor;
import lombok.Getter;

@Getter
@AllArgsConstructor
public enum CommandStatus {
    PENDING("pending"),
    RUNNING("running"),
    CANCELLING("cancelling"),
    DONE("done"),
    FAILED("failed"),
    INTERRUPTED("interrupted"),
    CANCELLED("cancelled");

    @JsonValue
    private final String value;

    @JsonCreator
    public static CommandStatus fromValue(String value) {
        for (CommandStatus candidate : values()) {
            if (candidate.value.equals(value)) {
                return candidate;
            }
        }
        throw new InvalidEnumValueException(CommandStatus.class, value);
    }
}
