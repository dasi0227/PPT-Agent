package com.dasi.ppt.agent.interfaces.controller.checkpoint.vo;

public record CheckpointPreviewVo(
        String revision,
        String inputPreview,
        long timestamp,
        int runCount
) {
}
