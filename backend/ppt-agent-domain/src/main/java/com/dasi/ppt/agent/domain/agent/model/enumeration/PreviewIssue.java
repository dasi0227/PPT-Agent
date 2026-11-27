package com.dasi.ppt.agent.domain.agent.model.enumeration;

import com.dasi.ppt.agent.domain.shared.exception.InvalidEnumValueException;
import com.fasterxml.jackson.annotation.JsonCreator;
import com.fasterxml.jackson.annotation.JsonValue;
import lombok.AllArgsConstructor;
import lombok.Getter;

@Getter
@AllArgsConstructor
public enum PreviewIssue {
    EXPRESSION_ISSUE("expression_issue"),
    LAYOUT_ISSUE("layout_issue"),
    VISUAL_ISSUE("visual_issue"),
    NONE("none");

    @JsonValue
    private final String value;

    @JsonCreator
    public static PreviewIssue fromValue(String value) {
        for (PreviewIssue candidate : values()) {
            if (candidate.value.equals(value)) {
                return candidate;
            }
        }
        throw new InvalidEnumValueException(PreviewIssue.class, value);
    }
}
