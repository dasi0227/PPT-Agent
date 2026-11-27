package com.dasi.ppt.agent.domain.agent.model.enumeration;

import com.dasi.ppt.agent.domain.shared.exception.InvalidEnumValueException;
import com.fasterxml.jackson.annotation.JsonCreator;
import com.fasterxml.jackson.annotation.JsonValue;
import lombok.AllArgsConstructor;
import lombok.Getter;

@Getter
@AllArgsConstructor
public enum ActionStatus {
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
    public static ActionStatus fromValue(String value) {
        for (ActionStatus candidate : values()) {
            if (candidate.value.equals(value)) {
                return candidate;
            }
        }
        throw new InvalidEnumValueException(ActionStatus.class, value);
    }
}
