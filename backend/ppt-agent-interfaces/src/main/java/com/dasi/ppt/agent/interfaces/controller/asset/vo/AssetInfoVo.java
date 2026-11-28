package com.dasi.ppt.agent.interfaces.controller.asset.vo;

import com.dasi.ppt.agent.domain.asset.model.entity.Asset;
import com.dasi.ppt.agent.domain.asset.model.enumeration.AssetType;
import lombok.Builder;
import lombok.Getter;

import java.util.List;

@Builder
@Getter
public final class AssetInfoVo {
    private String id;
    private AssetType type;
    private String name;
    private String description;
    private String content;
    private boolean enabled;
    private List<String> tagIds;
    private long updatedAt;

    public static AssetInfoVo toVo(Asset asset) {
        return AssetInfoVo.builder()
                .id(asset.getId())
                .type(asset.getType())
                .name(asset.getName())
                .description(asset.getDescription())
                .content(asset.getContent())
                .enabled(asset.getEnabled())
                .tagIds(asset.getTagIds())
                .updatedAt(asset.getUpdatedAt())
                .build();
    }
}
