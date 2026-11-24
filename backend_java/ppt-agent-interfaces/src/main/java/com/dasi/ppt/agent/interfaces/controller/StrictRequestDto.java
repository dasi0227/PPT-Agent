package com.dasi.ppt.agent.interfaces.controller;

import com.fasterxml.jackson.annotation.JsonAnySetter;

public interface StrictRequestDto {

    @JsonAnySetter
    default void rejectUnknownField(String field, Object value) {
        throw new IllegalArgumentException("请求包含未声明的字段：" + field + "=" +  value);
    }
}
