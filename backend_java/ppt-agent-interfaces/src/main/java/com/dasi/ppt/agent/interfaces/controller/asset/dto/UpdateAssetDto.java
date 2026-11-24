package com.dasi.ppt.agent.interfaces.controller.asset.dto;

import com.dasi.ppt.agent.domain.asset.model.entity.Asset;
import com.dasi.ppt.agent.interfaces.controller.dto.StrictRequestDto;
import com.dasi.ppt.agent.interfaces.validation.NotBlankIfPresent;
import jakarta.validation.constraints.AssertTrue;
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
public final class UpdateAssetDto implements StrictRequestDto {
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
    private String baseHash;
    private List<String> tagIds;
    private Boolean enabled;

    @AssertTrue(message = "更新资产需提供至少一个可修改字段")
    private boolean isUpdateProvided() {
        return name != null || description != null || content != null || tagIds != null || enabled != null;
    }

    @AssertTrue(message = "更新正文时需同时提交 content 和 base_hash")
    private boolean isContentPairedWithBaseHash() {
        return (content == null) == (baseHash == null);
    }

    public Asset toAsset() {
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
