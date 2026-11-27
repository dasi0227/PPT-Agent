package com.dasi.ppt.agent.domain.agent.model.enumeration;

import com.dasi.ppt.agent.domain.shared.exception.InvalidEnumValueException;
import com.fasterxml.jackson.annotation.JsonCreator;
import com.fasterxml.jackson.annotation.JsonValue;
import lombok.AllArgsConstructor;
import lombok.Getter;

@Getter
@AllArgsConstructor
public enum PreviewMismatch {
    META_MISMATCH("meta_mismatch"),
    CONTENT_MISMATCH("content_mismatch"),
    SPEC_MISMATCH("spec_mismatch"),
    VISUAL_MISMATCH("visual_mismatch"),
    NONE("none");

    @JsonValue
    private final String value;

    @JsonCreator
    public static PreviewMismatch fromValue(String value) {
        for (PreviewMismatch candidate : values()) {
            if (candidate.value.equals(value)) {
                return candidate;
            }
        }
        throw new InvalidEnumValueException(PreviewMismatch.class, value);
    }
}
