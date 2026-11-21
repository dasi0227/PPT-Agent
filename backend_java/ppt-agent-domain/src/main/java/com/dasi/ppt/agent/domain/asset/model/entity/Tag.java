package com.dasi.ppt.agent.domain.asset.model.entity;

import com.dasi.ppt.agent.domain.asset.model.enumeration.AssetType;

public record Tag(
        String id,
        AssetType type,
        String name
) {
}
