package com.dasi.ppt.agent.interfaces.controller.checkpoint.vo;

import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Getter;
import lombok.NoArgsConstructor;

import java.util.List;

@Builder
@Getter
@AllArgsConstructor
@NoArgsConstructor
public final class CheckpointListVo {
    private String revision;
    private List<String> runIds;
    private boolean canReturn;
}
