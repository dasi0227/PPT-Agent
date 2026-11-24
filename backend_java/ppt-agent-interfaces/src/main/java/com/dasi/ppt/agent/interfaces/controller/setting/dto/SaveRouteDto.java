package com.dasi.ppt.agent.interfaces.controller.setting.dto;

import com.dasi.ppt.agent.domain.setting.model.enumeration.ModelRoute;
import com.dasi.ppt.agent.interfaces.controller.StrictRequestDto;
import com.dasi.ppt.agent.interfaces.controller.NotBlankIfPresent;
import jakarta.validation.constraints.NotNull;
import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Getter;
import lombok.NoArgsConstructor;

@Builder
@Getter
@AllArgsConstructor
@NoArgsConstructor
public final class SaveRouteDto implements StrictRequestDto {
    @NotNull(message = "需指定 route")
    private ModelRoute route;
    @NotBlankIfPresent(message = "模型配置 ID 不能为空白")
    private String modelId;
}
