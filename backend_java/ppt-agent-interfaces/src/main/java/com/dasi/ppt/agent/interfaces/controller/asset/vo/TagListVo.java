package com.dasi.ppt.agent.interfaces.controller.asset.vo;

import java.util.List;

public record TagListVo(List<TagSummaryVo> items) {

    public record TagSummaryVo(String id, String name) {
    }
}
