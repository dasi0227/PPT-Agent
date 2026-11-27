package com.dasi.ppt.agent.interfaces.controller.asset.dto;

import com.dasi.ppt.agent.domain.asset.model.entity.Asset;
import com.dasi.ppt.agent.interfaces.controller.NotBlankIfPresent;
import jakarta.validation.constraints.NotBlank;
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
public final class UpdateAssetDto {
    @NotBlank(message = "更新资产需指定 asset_id")
    private String assetId;
    @NotBlankIfPresent(message = "资产名称不能为空白")
    @Size(max = 80, message = "资产名称最多 80 个字符")
    private String name;
    @NotBlankIfPresent(message = "资产描述不能为空白")
    @Size(max = 500, message = "资产描述最多 500 个字符")
    private String description;
    @Size(min = 1, message = "正文需为非空 UTF-8 原文")
    private String content;
    private List<String> tagIds;
    private Boolean enabled;

    public Asset toEntity() {
        return Asset.builder()
                .id(assetId)
                .name(name)
                .description(description)
                .content(content)
                .tagIds(tagIds)
                .enabled(enabled)
                .build();
    }
}
