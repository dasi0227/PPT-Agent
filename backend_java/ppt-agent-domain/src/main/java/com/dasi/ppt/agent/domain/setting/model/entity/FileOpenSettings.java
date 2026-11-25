package com.dasi.ppt.agent.domain.setting.model.entity;

import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Data;
import lombok.NoArgsConstructor;

import java.nio.file.Path;
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

    public static final String SYSTEM_OPEN = "system";
    public static final String INHERIT_OPEN = "inherit";

    public void normalize() {
        List<Path> apps = customApps.stream()
                .map(path -> Path.of(path).normalize())
                .distinct()
                .toList();

        defaultOpen = resolvePath(defaultOpen, apps, SYSTEM_OPEN);
        jsonOpen = resolvePath(jsonOpen, apps, INHERIT_OPEN);
        htmlOpen = resolvePath(htmlOpen, apps, INHERIT_OPEN);

        customApps = apps.stream()
                .map(Path::toString)
                .toList();
    }

    private static String resolvePath(String path, List<Path> apps, String fallback) {
        if (path.equals(SYSTEM_OPEN) || path.equals(fallback)) {
            return path;
        }
        int index = apps.indexOf(Path.of(path).normalize());
        return index >= 0 ? apps.get(index).toString() : fallback;
    }
}
