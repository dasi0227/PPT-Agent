package com.dasi.ppt.agent.domain.shared.sse;

import com.dasi.ppt.agent.domain.shared.exception.InvalidEnumValueException;
import com.fasterxml.jackson.annotation.JsonCreator;
import com.fasterxml.jackson.annotation.JsonValue;
import lombok.AllArgsConstructor;
import lombok.Getter;

@Getter
@AllArgsConstructor
public enum ProjectChangeArea {
    CONTENT("content"),
    APPEARANCE("appearance"),
    SESSIONS("sessions");

    @JsonValue
    private final String value;

    @JsonCreator
    public static ProjectChangeArea fromValue(String value) {
        for (ProjectChangeArea candidate : values()) {
            if (candidate.value.equals(value)) {
                return candidate;
            }
        }
        throw new InvalidEnumValueException(ProjectChangeArea.class, value);
    }
}
