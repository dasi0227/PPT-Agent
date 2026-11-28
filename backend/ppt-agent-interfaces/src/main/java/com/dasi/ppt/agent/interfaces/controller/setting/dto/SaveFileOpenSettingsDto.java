package com.dasi.ppt.agent.interfaces.controller.setting.dto;

import com.dasi.ppt.agent.domain.setting.model.entity.FileOpenSettings;
import jakarta.validation.constraints.NotBlank;
import jakarta.validation.constraints.NotNull;
import lombok.Getter;

import java.util.List;

@Getter
public final class SaveFileOpenSettingsDto {
    @NotBlank(message = "default_open 不能为空白")
    private String defaultOpen;
    @NotBlank(message = "json_open 不能为空白")
    private String jsonOpen;
    @NotBlank(message = "html_open 不能为空白")
    private String htmlOpen;
    @NotNull(message = "需提供 custom_apps")
    private List<@NotBlank(message = "custom_apps 中的应用路径不能为空白") String> customApps;

    public FileOpenSettings toEntity() {
        return FileOpenSettings.builder()
                .defaultOpen(defaultOpen)
                .jsonOpen(jsonOpen)
                .htmlOpen(htmlOpen)
                .customApps(customApps)
                .build();
    }
}
