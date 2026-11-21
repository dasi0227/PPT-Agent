package com.dasi.ppt.agent.interfaces.controller.setting.vo;

import com.dasi.ppt.agent.domain.setting.model.enumeration.LlmProtocol;

import java.util.List;

public record ModelSettingsVo(List<ModelConfigVo> models, ModelRoutesVo routes) {

    public record ModelConfigVo(
            String name,
            LlmProtocol protocol,
            String baseUrl,
            String model,
            String provider
    ) {
    }

    public record ModelRoutesVo(
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
