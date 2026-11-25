package com.dasi.ppt.agent.domain.asset.model.entity;

import com.dasi.ppt.agent.domain.asset.model.enumeration.AssetType;
import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Data;
import lombok.NoArgsConstructor;

import java.util.List;

@Builder
@Data
@AllArgsConstructor
@NoArgsConstructor
public final class Asset {
    private String id;
    private AssetType type;
    private String name;
    private String description;
    private String content;
    private Boolean enabled;
    private List<String> tagIds;
    private long updatedAt;
}
