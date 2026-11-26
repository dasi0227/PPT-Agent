package com.dasi.ppt.agent.infrastructure.persistence.database;

import com.dasi.ppt.agent.domain.shared.exception.InvalidEnumValueException;
import com.fasterxml.jackson.annotation.JsonCreator;
import com.fasterxml.jackson.annotation.JsonValue;
import lombok.AllArgsConstructor;
import lombok.Getter;

@Getter
@AllArgsConstructor
public enum ReceiptType {
    AGENT("agent"),
    MANUAL("manual");

    @JsonValue
    private final String value;

    @JsonCreator
    public static ReceiptType fromValue(String value) {
        for (ReceiptType candidate : values()) {
            if (candidate.value.equals(value)) {
                return candidate;
            }
        }
        throw new InvalidEnumValueException(ReceiptType.class, value);
    }
}
