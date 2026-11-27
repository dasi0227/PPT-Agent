package com.dasi.ppt.agent.domain.asset.model.enumeration;

import com.dasi.ppt.agent.domain.shared.exception.InvalidEnumValueException;
import com.fasterxml.jackson.annotation.JsonCreator;
import com.fasterxml.jackson.annotation.JsonValue;
import lombok.AllArgsConstructor;
import lombok.Getter;

@Getter
@AllArgsConstructor
public enum AssetType {
    THEME("theme"),
    COMPONENT("component"),
    SKILL("skill"),
    SNIPPET("snippet");

    @JsonValue
    private final String value;

    @JsonCreator
    public static AssetType fromValue(String value) {
        for (AssetType candidate : values()) {
            if (candidate.value.equals(value)) {
                return candidate;
            }
        }
        throw new InvalidEnumValueException(AssetType.class, value);
    }
}
