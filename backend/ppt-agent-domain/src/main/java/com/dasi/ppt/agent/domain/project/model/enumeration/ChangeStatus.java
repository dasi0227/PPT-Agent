package com.dasi.ppt.agent.domain.project.model.enumeration;

import com.dasi.ppt.agent.domain.shared.exception.InvalidEnumValueException;
import com.fasterxml.jackson.annotation.JsonCreator;
import com.fasterxml.jackson.annotation.JsonValue;
import lombok.AllArgsConstructor;
import lombok.Getter;

@Getter
@AllArgsConstructor
public enum ChangeStatus {
    ADDED("added"),
    MODIFIED("modified"),
    DELETED("deleted");

    @JsonValue
    private final String value;

    @JsonCreator
    public static ChangeStatus fromValue(String value) {
        for (ChangeStatus candidate : values()) {
            if (candidate.value.equals(value)) {
                return candidate;
            }
        }
        throw new InvalidEnumValueException(ChangeStatus.class, value);
    }
}
