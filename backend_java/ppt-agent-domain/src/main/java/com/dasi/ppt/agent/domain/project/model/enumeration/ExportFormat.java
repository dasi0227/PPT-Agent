package com.dasi.ppt.agent.domain.project.model.enumeration;

import com.dasi.ppt.agent.domain.shared.exception.InvalidEnumValueException;
import com.fasterxml.jackson.annotation.JsonCreator;
import com.fasterxml.jackson.annotation.JsonValue;
import lombok.AllArgsConstructor;
import lombok.Getter;

@Getter
@AllArgsConstructor
public enum ExportFormat {
    PNG("png"),
    PDF("pdf"),
    HTML("html");

    @JsonValue
    private final String value;

    @JsonCreator
    public static ExportFormat fromValue(String value) {
        for (ExportFormat candidate : values()) {
            if (candidate.value.equals(value)) {
                return candidate;
            }
        }
        throw new InvalidEnumValueException(ExportFormat.class, value);
    }
}
