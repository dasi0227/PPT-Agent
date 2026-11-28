package com.dasi.ppt.agent.interfaces.controller.asset.dto;

import com.dasi.ppt.agent.domain.asset.model.enumeration.AssetType;
import jakarta.validation.constraints.NotBlank;
import jakarta.validation.constraints.NotNull;
import lombok.Getter;

@Getter
public final class CreateTagDto {
    @NotNull(message = "创建标签需指定 type")
    private AssetType type;
    @NotBlank(message = "标签名称不能为空")
    private String name;
}
