package com.dasi.ppt.agent.domain.project.model.enumeration;

import com.dasi.ppt.agent.domain.shared.exception.InvalidEnumValueException;
import com.fasterxml.jackson.annotation.JsonCreator;
import com.fasterxml.jackson.annotation.JsonValue;
import lombok.AllArgsConstructor;
import lombok.Getter;

@Getter
@AllArgsConstructor
public enum SlidePurpose {
    COVER("cover"),
    INTRODUCTION("introduction"),
    TRANSITION("transition"),
    CONTENT_EXPLANATION("content_explanation"),
    CONTENT_COMPARISON("content_comparison"),
    CONTENT_EXAMPLE("content_example"),
    CONTENT_GUIDANCE("content_guidance"),
    CONTENT_OTHER("content_other"),
    CONCLUSION("conclusion"),
    OTHER("other");

    @JsonValue
    private final String value;

    @JsonCreator
    public static SlidePurpose fromValue(String value) {
        for (SlidePurpose candidate : values()) {
            if (candidate.value.equals(value)) {
                return candidate;
            }
        }
        throw new InvalidEnumValueException(SlidePurpose.class, value);
    }
}
