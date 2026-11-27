package com.dasi.ppt.agent.domain.project.model.enumeration;

import com.dasi.ppt.agent.domain.shared.exception.InvalidEnumValueException;
import com.fasterxml.jackson.annotation.JsonCreator;
import com.fasterxml.jackson.annotation.JsonValue;
import lombok.AllArgsConstructor;
import lombok.Getter;

@Getter
@AllArgsConstructor
public enum DiffRowType {
    ADDED("added"),
    REMOVED("removed"),
    CONTEXT("context");

    @JsonValue
    private final String value;

    @JsonCreator
    public static DiffRowType fromValue(String value) {
        for (DiffRowType candidate : values()) {
            if (candidate.value.equals(value)) {
                return candidate;
            }
        }
        throw new InvalidEnumValueException(DiffRowType.class, value);
    }
}
