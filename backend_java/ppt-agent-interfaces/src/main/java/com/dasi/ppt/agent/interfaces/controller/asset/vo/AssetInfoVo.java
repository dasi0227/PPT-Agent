package com.dasi.ppt.agent.interfaces.controller.asset.vo;

import com.dasi.ppt.agent.domain.asset.model.entity.Asset;
import com.dasi.ppt.agent.domain.asset.model.enumeration.AssetType;
import com.dasi.ppt.agent.domain.shared.util.HashUtil;
import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Getter;
import lombok.NoArgsConstructor;

import java.util.List;

@Builder
@Getter
@AllArgsConstructor
@NoArgsConstructor
public final class AssetInfoVo {
    private String id;
    private AssetType type;
    private String name;
    private String description;
    private boolean enabled;
    private List<String> tagIds;
    private long updatedAt;
    private String hash;

    public static AssetInfoVo toVo(Asset asset) {
        return AssetInfoVo.builder()
                .id(asset.getId())
                .type(asset.getType())
                .name(asset.getName())
                .description(asset.getDescription())
                .enabled(asset.getEnabled())
                .tagIds(asset.getTagIds())
                .updatedAt(asset.getUpdatedAt())
                .hash(asset.getContent() == null ? null : HashUtil.calAssetHash(asset))
                .build();
    }
}
