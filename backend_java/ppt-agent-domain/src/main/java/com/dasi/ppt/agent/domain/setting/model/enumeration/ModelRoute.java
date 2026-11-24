package com.dasi.ppt.agent.domain.setting.model.enumeration;

import com.dasi.ppt.agent.domain.shared.exception.InvalidEnumValueException;
import com.fasterxml.jackson.annotation.JsonCreator;
import com.fasterxml.jackson.annotation.JsonValue;
import lombok.AllArgsConstructor;
import lombok.Getter;

@Getter
@AllArgsConstructor
public enum ModelRoute {
    MAIN_DEFAULT("main_default"),
    MAIN_FALLBACK("main_fallback"),
    SIDE_DEFAULT("side_default"),
    SIDE_FALLBACK("side_fallback"),
    RENAME("rename"),
    COMPACT("compact"),
    COMMIT("commit"),
    POLISH("polish"),
    HANDOFF("handoff"),
    SUGGEST("suggest");

    @JsonValue
    private final String value;

    @JsonCreator
    public static ModelRoute fromValue(String value) {
        for (ModelRoute candidate : values()) {
            if (candidate.value.equals(value)) {
                return candidate;
            }
        }
        throw new InvalidEnumValueException(ModelRoute.class, value);
    }
}
