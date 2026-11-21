package com.dasi.ppt.agent.domain.asset.model.entity;

import com.dasi.ppt.agent.domain.asset.model.enumeration.AssetType;

import java.util.List;

public record Asset(
        String id,
        AssetType type,
        String name,
        String description,
        String content,
        boolean disabled,
        List<String> tagIds,
        long updatedAt
) {
}
