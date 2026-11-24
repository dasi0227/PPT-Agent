package com.dasi.ppt.agent.domain.agent.model.enumeration;

import com.dasi.ppt.agent.domain.shared.exception.InvalidEnumValueException;
import com.fasterxml.jackson.annotation.JsonCreator;
import com.fasterxml.jackson.annotation.JsonValue;
import lombok.AllArgsConstructor;
import lombok.Getter;

@Getter
@AllArgsConstructor
public enum RunStatus {
    PENDING("pending"),
    RUNNING("running"),
    WAITING("waiting"),
    CANCELLING("cancelling"),
    DONE("done"),
    REJECTED("rejected"),
    FAILED("failed"),
    INTERRUPTED("interrupted"),
    CANCELLED("cancelled");

    @JsonValue
    private final String value;

    @JsonCreator
    public static RunStatus fromValue(String value) {
        for (RunStatus candidate : values()) {
            if (candidate.value.equals(value)) {
                return candidate;
            }
        }
        throw new InvalidEnumValueException(RunStatus.class, value);
    }
}
