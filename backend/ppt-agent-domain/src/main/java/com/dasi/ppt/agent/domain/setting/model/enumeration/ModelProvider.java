package com.dasi.ppt.agent.domain.setting.model.enumeration;

import com.dasi.ppt.agent.domain.shared.exception.InvalidEnumValueException;
import com.fasterxml.jackson.annotation.JsonCreator;
import com.fasterxml.jackson.annotation.JsonValue;
import lombok.AllArgsConstructor;
import lombok.Getter;

@Getter
@AllArgsConstructor
public enum ModelProvider {
    DEEPSEEK("deepseek"),
    KIMI("kimi"),
    MIMO("mimo"),
    GPT("gpt"),
    CLAUDE("claude"),
    GEMINI("gemini"),
    QWEN("qwen"),
    MINIMAX("minimax"),
    GLM("glm"),
    GROK("grok");

    @JsonValue
    private final String value;

    @JsonCreator
    public static ModelProvider fromValue(String value) {
        for (ModelProvider candidate : values()) {
            if (candidate.value.equals(value)) {
                return candidate;
            }
        }
        throw new InvalidEnumValueException(ModelProvider.class, value);
    }
}
