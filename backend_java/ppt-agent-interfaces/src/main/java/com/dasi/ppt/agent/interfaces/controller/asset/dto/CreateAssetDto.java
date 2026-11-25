package com.dasi.ppt.agent.interfaces.controller.asset.dto;

import com.dasi.ppt.agent.domain.asset.model.entity.Asset;
import com.dasi.ppt.agent.domain.asset.model.enumeration.AssetType;
import jakarta.validation.constraints.NotBlank;
import jakarta.validation.constraints.NotEmpty;
import jakarta.validation.constraints.NotNull;
import jakarta.validation.constraints.Size;
import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Getter;
import lombok.NoArgsConstructor;

import java.util.List;

@Builder
@Getter
@AllArgsConstructor
@NoArgsConstructor
public final class CreateAssetDto {
    @NotNull(message = "创建资产需指定 type")
    private AssetType type;
    @NotBlank(message = "资产名称不能为空白")
    @Size(max = 80, message = "资产名称最多 80 个字符")
    private String name;
    @NotBlank(message = "资产描述不能为空白")
    @Size(max = 500, message = "资产描述最多 500 个字符")
    private String description;
    @NotEmpty(message = "正文需为非空 UTF-8 原文")
    private String content;
    private List<String> tagIds;

    public Asset toEntity() {
        return Asset.builder()
                .type(type)
                .name(name)
                .description(description)
                .content(content)
                .tagIds(tagIds)
                .build();
    }
}
