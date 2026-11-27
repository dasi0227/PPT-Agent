package com.dasi.ppt.agent.domain.project.model.enumeration;

import com.dasi.ppt.agent.domain.shared.exception.InvalidEnumValueException;
import com.fasterxml.jackson.annotation.JsonCreator;
import com.fasterxml.jackson.annotation.JsonValue;
import lombok.AllArgsConstructor;
import lombok.Getter;

@Getter
@AllArgsConstructor
public enum OutlineNodeType {
    CHAPTER("chapter"),
    SECTION("section"),
    SLIDE("slide");

    @JsonValue
    private final String value;

    @JsonCreator
    public static OutlineNodeType fromValue(String value) {
        for (OutlineNodeType candidate : values()) {
            if (candidate.value.equals(value)) {
                return candidate;
            }
        }
        throw new InvalidEnumValueException(OutlineNodeType.class, value);
    }
}
