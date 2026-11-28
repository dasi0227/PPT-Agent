package com.dasi.ppt.agent.interfaces.controller.asset.vo;

import lombok.Builder;
import lombok.Getter;

import java.util.List;

@Getter
@Builder
public final class AssetListVo {
    private List<AssetInfoVo> items;
}
