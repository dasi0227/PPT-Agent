package com.dasi.ppt.agent.domain.setting.model.entity;

import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Data;
import lombok.NoArgsConstructor;

import java.util.List;

@Builder
@Data
@AllArgsConstructor
@NoArgsConstructor
public final class FileOpenSettings {
    private String defaultOpen;
    private String jsonOpen;
    private String htmlOpen;
    private List<String> customApps;
}
