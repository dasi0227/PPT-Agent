package com.dasi.ppt.agent.domain.project.model.enumeration;

import com.dasi.ppt.agent.domain.shared.exception.InvalidEnumValueException;
import com.fasterxml.jackson.annotation.JsonCreator;
import com.fasterxml.jackson.annotation.JsonValue;
import lombok.AllArgsConstructor;
import lombok.Getter;

@Getter
@AllArgsConstructor
public enum ExportStatus {
    SNAPSHOTTING("snapshotting"),
    RENDERING("rendering"),
    PACKAGING("packaging"),
    READY("ready"),
    FAILED("failed"),
    CANCELED("canceled");

    @JsonValue
    private final String value;

    @JsonCreator
    public static ExportStatus fromValue(String value) {
        for (ExportStatus candidate : values()) {
            if (candidate.value.equals(value)) {
                return candidate;
            }
        }
        throw new InvalidEnumValueException(ExportStatus.class, value);
    }
}
