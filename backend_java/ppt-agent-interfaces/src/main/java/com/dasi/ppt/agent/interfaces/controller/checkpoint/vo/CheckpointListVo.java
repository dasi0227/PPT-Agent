package com.dasi.ppt.agent.interfaces.controller.checkpoint.vo;

import java.util.List;

public record CheckpointListVo(
        String revision,
        List<String> runIds,
        boolean canReturn
) {
}
