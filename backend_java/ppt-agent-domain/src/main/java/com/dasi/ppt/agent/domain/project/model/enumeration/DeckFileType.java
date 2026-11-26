package com.dasi.ppt.agent.domain.project.model.enumeration;

import com.dasi.ppt.agent.domain.shared.exception.InvalidEnumValueException;
import com.fasterxml.jackson.annotation.JsonCreator;
import com.fasterxml.jackson.annotation.JsonValue;
import lombok.AllArgsConstructor;
import lombok.Getter;

@Getter
@AllArgsConstructor
public enum DeckFileType {
    META("meta"),
    CONSTRAINT("constraint"),
    OUTLINE("outline"),
    SPEC("spec"),
    HTML("html");

    @JsonValue
    private final String value;

    @JsonCreator
    public static DeckFileType fromValue(String value) {
        for (DeckFileType candidate : values()) {
            if (candidate.value.equals(value)) {
                return candidate;
            }
        }
        throw new InvalidEnumValueException(DeckFileType.class, value);
    }
}
