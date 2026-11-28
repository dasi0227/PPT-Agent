package com.dasi.ppt.agent.interfaces.controller.checkpoint.vo;

import lombok.Builder;
import lombok.Getter;

@Builder
@Getter
public final class CheckpointPreviewVo {
    private String revision;
    private String inputPreview;
    private long timestamp;
    private int runCount;
}
