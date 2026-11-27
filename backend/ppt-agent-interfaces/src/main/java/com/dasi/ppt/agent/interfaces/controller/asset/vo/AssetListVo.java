package com.dasi.ppt.agent.interfaces.controller.asset.vo;

import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Getter;
import lombok.NoArgsConstructor;

import java.util.List;

@Getter
@Builder
@AllArgsConstructor
@NoArgsConstructor
public final class AssetListVo {
    private List<AssetInfoVo> items;
}
