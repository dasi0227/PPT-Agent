package com.dasi.ppt.agent.domain.agent.model.enumeration;

import com.dasi.ppt.agent.domain.shared.exception.InvalidEnumValueException;
import com.fasterxml.jackson.annotation.JsonCreator;
import com.fasterxml.jackson.annotation.JsonValue;
import lombok.AllArgsConstructor;
import lombok.Getter;

@Getter
@AllArgsConstructor
public enum CommandType {
    RENAME("rename"),
    POLISH("polish"),
    HANDOFF("handoff"),
    COMPACT("compact"),
    COMMIT("commit"),
    SUGGEST("suggest");

    @JsonValue
    private final String value;

    @JsonCreator
    public static CommandType fromValue(String value) {
        for (CommandType candidate : values()) {
            if (candidate.value.equals(value)) {
                return candidate;
            }
        }
        throw new InvalidEnumValueException(CommandType.class, value);
    }
}
