package com.dasi.ppt.agent.domain.shared.exception;

public class InvalidEnumValueException extends IllegalArgumentException {

    public InvalidEnumValueException(Class<? extends Enum<?>> type, String value) {
        super(type.getSimpleName() + " 的取值无效：" + value);
    }
}
