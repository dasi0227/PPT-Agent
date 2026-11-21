package com.dasi.ppt.agent.interfaces.handler;

public record ErrorResponse(Error error) {

    public record Error(String code, String message) {
    }
}
