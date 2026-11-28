package com.dasi.ppt.agent.interfaces.controller.setting.dto;

import jakarta.validation.constraints.NotBlank;
import lombok.Getter;

@Getter
public final class DeleteModelDto {
    @NotBlank(message = "删除模型需指定 id")
    private String id;
}
