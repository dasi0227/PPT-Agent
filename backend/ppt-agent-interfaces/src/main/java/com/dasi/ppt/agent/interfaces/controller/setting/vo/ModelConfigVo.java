package com.dasi.ppt.agent.interfaces.controller.setting.vo;

import com.dasi.ppt.agent.domain.setting.model.entity.ModelConfigSettings;
import com.dasi.ppt.agent.domain.setting.model.enumeration.LlmProtocol;
import com.dasi.ppt.agent.domain.setting.model.enumeration.ModelProvider;
import lombok.Builder;
import lombok.Getter;

@Builder
@Getter
public final class ModelConfigVo {
    private String id;
    private String name;
    private LlmProtocol protocol;
    private String baseUrl;
    private String model;
    private ModelProvider provider;

    public static ModelConfigVo toVo(ModelConfigSettings config) {
        return ModelConfigVo.builder()
                .id(config.getId())
                .name(config.getName())
                .protocol(config.getProtocol())
                .baseUrl(config.getBaseUrl())
                .model(config.getModel())
                .provider(config.getProvider())
                .build();
    }
}
