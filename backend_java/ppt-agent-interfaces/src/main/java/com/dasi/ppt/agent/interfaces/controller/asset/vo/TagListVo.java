package com.dasi.ppt.agent.interfaces.controller.asset.vo;

import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Getter;
import lombok.NoArgsConstructor;

import java.util.List;

@Builder
@Getter
@AllArgsConstructor
@NoArgsConstructor
public final class TagListVo {
    private List<TagInfoVo> items;
}
