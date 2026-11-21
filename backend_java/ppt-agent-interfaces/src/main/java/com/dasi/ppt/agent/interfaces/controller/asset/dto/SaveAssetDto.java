package com.dasi.ppt.agent.interfaces.controller.asset.dto;

import com.dasi.ppt.agent.domain.asset.model.enumeration.AssetType;

import java.util.List;

public record SaveAssetDto(
        String assetId,
        AssetType type,
        String name,
        String description,
        String content,
        String baseHash,
        List<String> tagIds,
        Boolean disabled
) {
}
