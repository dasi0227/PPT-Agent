package com.dasi.ppt.agent.domain.agent.service.tool.result;

import com.dasi.ppt.agent.domain.asset.model.enumeration.AssetType;
import lombok.Builder;
import lombok.Getter;

@Builder
@Getter
public final class LoadedResource {

    private final AssetType type;
    private final String id;
    private final String name;
}
