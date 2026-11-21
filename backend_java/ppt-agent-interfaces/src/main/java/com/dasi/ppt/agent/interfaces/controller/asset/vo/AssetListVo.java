package com.dasi.ppt.agent.interfaces.controller.asset.vo;

import com.dasi.ppt.agent.domain.asset.model.enumeration.AssetType;

import java.util.List;

public record AssetListVo(List<AssetSummaryVo> items) {

    public record AssetSummaryVo(
            String id,
            AssetType type,
            String name,
            String description,
            boolean disabled,
            List<String> tagIds,
            long updatedAt
    ) {
    }
}
