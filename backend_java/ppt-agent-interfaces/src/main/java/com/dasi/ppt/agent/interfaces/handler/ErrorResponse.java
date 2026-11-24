package com.dasi.ppt.agent.interfaces.handler;

import com.dasi.ppt.agent.domain.shared.exception.ErrorCode;
import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Getter;
import lombok.NoArgsConstructor;

@Builder
@Getter
@AllArgsConstructor
@NoArgsConstructor
public final class ErrorResponse {
    private Error error;

    public static ErrorResponse of(ErrorCode code) {
        return of(code, code.getDefaultMessage());
    }

    public static ErrorResponse of(ErrorCode code, String message) {
        return ErrorResponse.builder()
                .error(Error.builder()
                        .code(code.getValue())
                        .message(message)
                        .build())
                .build();
    }

    @Builder
    @Getter
    @AllArgsConstructor
    @NoArgsConstructor
    public static final class Error {
        private String code;
        private String message;
    }
}
