package com.dasi.ppt.agent.interfaces.controller.setting.vo;

import com.dasi.ppt.agent.domain.setting.model.entity.FileOpenSettings;
import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Getter;
import lombok.NoArgsConstructor;

import java.util.List;

@Builder
@Getter
@AllArgsConstructor
@NoArgsConstructor
public final class FileOpenSettingsVo {
    private String defaultOpen;
    private String jsonOpen;
    private String htmlOpen;
    private List<String> customApps;

    public static FileOpenSettingsVo toVo(FileOpenSettings settings) {
        return FileOpenSettingsVo.builder()
                .defaultOpen(settings.getDefaultOpen())
                .jsonOpen(settings.getJsonOpen())
                .htmlOpen(settings.getHtmlOpen())
                .customApps(settings.getCustomApps())
                .build();
    }
}
