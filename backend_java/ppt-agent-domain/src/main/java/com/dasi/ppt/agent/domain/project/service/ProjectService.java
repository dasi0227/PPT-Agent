package com.dasi.ppt.agent.domain.project.service;

import com.dasi.ppt.agent.domain.project.repository.IProjectRepository;
import com.dasi.ppt.agent.domain.gateway.IPlatformGateway;
import com.dasi.ppt.agent.domain.setting.model.entity.FileOpenSettings;
import com.dasi.ppt.agent.domain.setting.service.SettingService;
import com.dasi.ppt.agent.domain.shared.sse.EventPublisher;
import lombok.RequiredArgsConstructor;
import org.springframework.stereotype.Service;

import java.nio.file.Path;
import java.util.Locale;

import static com.dasi.ppt.agent.domain.setting.model.entity.FileOpenSettings.INHERIT_OPEN;
import static com.dasi.ppt.agent.domain.setting.model.entity.FileOpenSettings.SYSTEM_OPEN;

@Service
@RequiredArgsConstructor
public class ProjectService {

    private final IProjectRepository projectRepository;
    private final ProjectWriteLock projectWriteLock;
    private final EventPublisher eventPublisher;
    private final SettingService settingService;
    private final IPlatformGateway platformGateway;

    public void openFile(String projectId, String relativePath) {
        FileOpenSettings settings = settingService.getFileOpenSettings();
        String filename = relativePath.toLowerCase(Locale.ROOT);
        String choice = settings.getDefaultOpen();
        if (filename.endsWith(".json")) {
            choice = settings.getJsonOpen();
        } else if (filename.endsWith(".html") || filename.endsWith(".htm")) {
            choice = settings.getHtmlOpen();
        }
        if (INHERIT_OPEN.equals(choice)) {
            choice = settings.getDefaultOpen();
        }
        Path application = SYSTEM_OPEN.equals(choice) ? null : Path.of(choice);
        platformGateway.openFile(projectId, relativePath, application);
    }
}
