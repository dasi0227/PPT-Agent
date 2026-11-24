package com.dasi.ppt.agent.interfaces.controller.asset.vo;

import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Getter;
import lombok.NoArgsConstructor;

@Builder
@Getter
@AllArgsConstructor
@NoArgsConstructor
public final class AssetContentVo {
    private String content;
    private String hash;
}
