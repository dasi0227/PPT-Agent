package com.dasi.ppt.agent.interfaces.controller.setting.dto;

import com.dasi.ppt.agent.domain.setting.model.entity.ModelConfigSettings;
import com.dasi.ppt.agent.domain.setting.model.enumeration.LlmProtocol;
import com.dasi.ppt.agent.domain.setting.model.enumeration.ModelProvider;
import com.dasi.ppt.agent.interfaces.controller.dto.StrictRequestDto;
import com.dasi.ppt.agent.interfaces.validation.NotBlankIfPresent;
import jakarta.validation.constraints.NotBlank;
import jakarta.validation.constraints.Size;
import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Getter;
import lombok.NoArgsConstructor;

@Builder
@Getter
@AllArgsConstructor
@NoArgsConstructor
public final class UpdateModelDto implements StrictRequestDto {
    @NotBlank(message = "更新模型需指定 id")
    private String id;
    @NotBlankIfPresent(message = "模型配置名不能为空白")
    @Size(max = 80, message = "模型配置名最多 80 个字符")
    private String name;
    private LlmProtocol protocol;
    private ModelProvider provider;
    @NotBlankIfPresent(message = "base_url 不能为空白")
    private String baseUrl;
    @NotBlankIfPresent(message = "上游模型名不能为空白")
    private String model;
    @NotBlankIfPresent(message = "模型密钥不能为空白")
    private String key;

    public ModelConfigSettings toModelConfigSettings() {
        return ModelConfigSettings.builder()
                .id(id)
                .name(name)
                .protocol(protocol)
                .provider(provider)
                .baseUrl(baseUrl)
                .model(model)
                .key(key)
                .build();
    }
}
