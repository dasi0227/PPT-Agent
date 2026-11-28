package com.dasi.ppt.agent.interfaces.controller.checkpoint.vo;

import lombok.Builder;
import lombok.Getter;

import java.util.List;

@Builder
@Getter
public final class CheckpointListVo {
    private String revision;
    private List<String> runIds;
    private boolean canReturn;
}
