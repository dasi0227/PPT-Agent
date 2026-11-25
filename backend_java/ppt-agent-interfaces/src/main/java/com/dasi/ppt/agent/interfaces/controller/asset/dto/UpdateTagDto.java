package com.dasi.ppt.agent.interfaces.controller.asset.dto;

import jakarta.validation.constraints.NotBlank;
import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Getter;
import lombok.NoArgsConstructor;

@Builder
@Getter
@AllArgsConstructor
@NoArgsConstructor
public final class UpdateTagDto {
    @NotBlank(message = "缺少 tag_id")
    private String tagId;
    @NotBlank(message = "标签名称不能为空")
    private String name;
}
