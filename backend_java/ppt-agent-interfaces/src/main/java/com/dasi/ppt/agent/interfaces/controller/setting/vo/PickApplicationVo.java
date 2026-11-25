package com.dasi.ppt.agent.interfaces.controller.setting.vo;

import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Getter;
import lombok.NoArgsConstructor;

import java.nio.file.Path;

@Builder
@Getter
@AllArgsConstructor
@NoArgsConstructor
public final class PickApplicationVo {
    private String path;

    public static PickApplicationVo toVo(Path path) {
        return PickApplicationVo.builder()
                .path(path == null ? null : path.toString())
                .build();
    }
}
