package com.dasi.ppt.agent.interfaces.controller.setting.dto;

import com.dasi.ppt.agent.domain.setting.model.enumeration.LlmProtocol;
import com.fasterxml.jackson.annotation.JsonProperty;

import java.util.List;

public record SaveModelSettingsDto(List<ModelConfigDto> models, ModelRoutesDto routes) {

    public record ModelConfigDto(
            String name,
            LlmProtocol protocol,
            String baseUrl,
            String model,
            String previousName,
            @JsonProperty(access = JsonProperty.Access.WRITE_ONLY) String key
    ) {
    }

    public record ModelRoutesDto(
            String mainDefault,
            String mainFallback,
            String sideDefault,
            String sideFallback,
            String rename,
            String compact,
            String commit,
            String polish,
            String handoff,
            String suggest
    ) {
    }
}
