package com.dasi.ppt.agent.domain.shared.exception;

import lombok.Getter;

@Getter
public class BusinessException extends RuntimeException {

    private final ErrorCode code;

    public static BusinessException invalid() {
        return invalid(ErrorCode.INVALID_ARGUMENT.getDefaultMessage());
    }

    public static BusinessException invalid(String message) {
        return new BusinessException(ErrorCode.INVALID_ARGUMENT, message);
    }

    public static BusinessException notFound(ErrorCode code) {
        return notFound(code, code.getDefaultMessage());
    }

    public static BusinessException notFound(ErrorCode code, String message) {
        return new BusinessException(code, message);
    }

    public static BusinessException conflict(ErrorCode code) {
        return conflict(code, code.getDefaultMessage());
    }

    public static BusinessException conflict(ErrorCode code, String message) {
        return new BusinessException(code, message);
    }

    private BusinessException(ErrorCode code, String message) {
        super(message);
        this.code = code;
    }

    public static BusinessException failed(ErrorCode code, String message) {
        return new BusinessException(code, message);
    }
}
