package com.dasi.ppt.agent.interfaces.controller.setting.dto;

import com.dasi.ppt.agent.domain.setting.model.entity.ModelConfigSettings;
import com.dasi.ppt.agent.domain.setting.model.enumeration.LlmProtocol;
import com.dasi.ppt.agent.domain.setting.model.enumeration.ModelProvider;
import jakarta.validation.constraints.NotBlank;
import jakarta.validation.constraints.NotNull;
import jakarta.validation.constraints.Size;
import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Getter;
import lombok.NoArgsConstructor;

@Builder
@Getter
@AllArgsConstructor
@NoArgsConstructor
public final class CreateModelDto {
    @NotBlank(message = "模型配置名不能为空白")
    @Size(max = 80, message = "模型配置名最多 80 个字符")
    private String name;
    @NotNull(message = "创建模型需指定 protocol")
    private LlmProtocol protocol;
    @NotNull(message = "创建模型需指定 provider")
    private ModelProvider provider;
    @NotBlank(message = "创建模型需指定 base_url")
    private String baseUrl;
    @NotBlank(message = "上游模型名不能为空白")
    private String model;
    @NotBlank(message = "模型密钥不能为空白")
    private String key;

    public ModelConfigSettings toModelConfigSettings() {
        return ModelConfigSettings.builder()
                .name(name)
                .protocol(protocol)
                .provider(provider)
                .baseUrl(baseUrl)
                .model(model)
                .key(key)
                .build();
    }
}
