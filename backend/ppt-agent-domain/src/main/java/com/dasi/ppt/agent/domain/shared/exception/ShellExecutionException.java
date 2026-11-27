package com.dasi.ppt.agent.domain.shared.exception;

import lombok.Getter;

@Getter
public class ShellExecutionException extends RuntimeException {

    private final String code;

    public ShellExecutionException(String code, String message, Throwable cause) {
        super(message, cause);
        this.code = code;
    }
}
