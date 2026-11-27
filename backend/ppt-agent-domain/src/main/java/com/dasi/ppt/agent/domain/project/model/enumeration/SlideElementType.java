package com.dasi.ppt.agent.domain.project.model.enumeration;

import com.dasi.ppt.agent.domain.shared.exception.InvalidEnumValueException;
import com.fasterxml.jackson.annotation.JsonCreator;
import com.fasterxml.jackson.annotation.JsonValue;
import lombok.AllArgsConstructor;
import lombok.Getter;

@Getter
@AllArgsConstructor
public enum SlideElementType {
    TEXT("text"),
    LIST("list"),
    METRIC("metric"),
    QUOTE("quote"),
    TABLE("table"),
    CHART("chart"),
    DIAGRAM("diagram"),
    CODE("code"),
    ASSET("asset");

    @JsonValue
    private final String value;

    @JsonCreator
    public static SlideElementType fromValue(String value) {
        for (SlideElementType candidate : values()) {
            if (candidate.value.equals(value)) {
                return candidate;
            }
        }
        throw new InvalidEnumValueException(SlideElementType.class, value);
    }
}
