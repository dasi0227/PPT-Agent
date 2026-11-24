package com.dasi.ppt.agent.interfaces.controller.checkpoint.vo;

import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Getter;
import lombok.NoArgsConstructor;

@Builder
@Getter
@AllArgsConstructor
@NoArgsConstructor
public final class CheckpointPreviewVo {
    private String revision;
    private String inputPreview;
    private long timestamp;
    private int runCount;
}
