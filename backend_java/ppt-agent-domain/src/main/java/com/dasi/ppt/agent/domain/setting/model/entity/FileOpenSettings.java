package com.dasi.ppt.agent.domain.setting.model.entity;

import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Data;
import lombok.NoArgsConstructor;

import java.nio.file.InvalidPathException;
import java.nio.file.Path;
import java.util.List;
import java.util.Locale;

import static com.dasi.ppt.agent.domain.shared.exception.BusinessException.invalid;

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
                .map(FileOpenSettings::resolveAppPath)
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
        int index = apps.indexOf(resolveAppPath(path));
        return index >= 0 ? apps.get(index).toString() : fallback;
    }

    private static Path resolveAppPath(String path) {
        Path normalized;
        try {
            normalized = Path.of(path).normalize();
        } catch (InvalidPathException e) {
            throw invalid("应用路径格式无效");
        }
        if (!normalized.isAbsolute()) {
            throw invalid("仅支持绝对路径");
        }

        String normalizedPath = normalized.toString();
        boolean validExtension = System.getProperty("os.name").startsWith("Windows")
                ? normalizedPath.toLowerCase(Locale.ROOT).endsWith(".exe")
                : normalizedPath.toLowerCase(Locale.ROOT).endsWith(".app");
        if (!validExtension) {
            throw invalid("仅支持应用程序");
        }
        return normalized;
    }
}
