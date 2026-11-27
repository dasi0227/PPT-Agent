package com.dasi.ppt.agent.interfaces.controller.setting.vo;

import com.dasi.ppt.agent.domain.setting.model.entity.ModelSettings;
import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Getter;
import lombok.NoArgsConstructor;

import java.util.List;

@Builder
@Getter
@AllArgsConstructor
@NoArgsConstructor
public final class ModelSettingsVo {
    private List<ModelConfigVo> models;
    private ModelRoutesVo routes;

    public static ModelSettingsVo toVo(ModelSettings settings) {
        List<ModelConfigVo> models = settings.getModels()
                .stream()
                .map(ModelConfigVo::toVo)
                .toList();
        return ModelSettingsVo.builder()
                .models(models)
                .routes(ModelRoutesVo.toVo(settings.getRoutes()))
                .build();
    }
}
