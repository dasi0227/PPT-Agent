package com.dasi.ppt.agent.domain.project.model.enumeration;

import com.dasi.ppt.agent.domain.shared.exception.InvalidEnumValueException;
import com.fasterxml.jackson.annotation.JsonCreator;
import com.fasterxml.jackson.annotation.JsonValue;
import lombok.AllArgsConstructor;
import lombok.Getter;

@Getter
@AllArgsConstructor
public enum DecorationType {
    SLIDE_PAGINATION("slide_pagination"),
    DECK_TITLE("deck_title"),
    CHAPTER_TITLE("chapter_title"),
    LOGO("logo");

    @JsonValue
    private final String value;

    @JsonCreator
    public static DecorationType fromValue(String value) {
        for (DecorationType candidate : values()) {
            if (candidate.value.equals(value)) {
                return candidate;
            }
        }
        throw new InvalidEnumValueException(DecorationType.class, value);
    }
}
