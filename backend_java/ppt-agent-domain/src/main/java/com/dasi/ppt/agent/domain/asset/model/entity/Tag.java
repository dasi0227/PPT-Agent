package com.dasi.ppt.agent.domain.asset.model.entity;

import com.dasi.ppt.agent.domain.asset.model.enumeration.AssetType;
import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Data;
import lombok.NoArgsConstructor;

@Builder
@Data
@AllArgsConstructor
@NoArgsConstructor
public final class Tag {
    private String id;
    private AssetType type;
    private String name;
}
