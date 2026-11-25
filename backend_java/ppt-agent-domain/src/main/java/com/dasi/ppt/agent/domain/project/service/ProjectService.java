package com.dasi.ppt.agent.domain.project.service;

import com.dasi.ppt.agent.domain.asset.model.entity.Asset;
import com.dasi.ppt.agent.domain.asset.model.enumeration.AssetType;
import com.dasi.ppt.agent.domain.asset.repository.IAssetRepository;
import com.dasi.ppt.agent.domain.shared.gateway.IPlatformGateway;
import com.dasi.ppt.agent.domain.project.model.entity.Project;
import com.dasi.ppt.agent.domain.project.repository.IProjectRepository;
import com.dasi.ppt.agent.domain.setting.model.entity.FileOpenSettings;
import com.dasi.ppt.agent.domain.setting.service.SettingService;
import com.dasi.ppt.agent.domain.shared.exception.ErrorCode;
import com.dasi.ppt.agent.domain.shared.sse.EventPublisher;
import com.dasi.ppt.agent.domain.shared.sse.SseEvent;
import com.dasi.ppt.agent.domain.shared.util.IdUtil;
import lombok.RequiredArgsConstructor;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;
import org.springframework.transaction.support.TransactionTemplate;

import java.nio.file.Path;
import java.util.List;
import java.util.Locale;
import java.util.Map;

import static com.dasi.ppt.agent.domain.shared.constant.SystemConstant.SYSTEM_OPEN;
import static com.dasi.ppt.agent.domain.shared.constant.SystemConstant.INHERIT_OPEN;
import static com.dasi.ppt.agent.domain.shared.exception.BusinessException.invalid;
import static com.dasi.ppt.agent.domain.shared.exception.BusinessException.notFound;

@Service
@RequiredArgsConstructor
public class ProjectService {

    private final IProjectRepository projectRepository;
    private final IAssetRepository assetRepository;
    private final TransactionTemplate transaction;
    private final ProjectWriteLock projectWriteLock;
    private final EventPublisher eventPublisher;
    private final SettingService settingService;
    private final IPlatformGateway platformGateway;

    public List<Project> listProjects() {
        return projectRepository.listProjects();
    }

    public Project findAppearance(String projectId) {
        return projectRepository.findAppearance(projectId);
    }

    public byte[] findLogo(String projectId) {
        return projectRepository.findLogo(projectId);
    }

    @Transactional
    public Project createProject(String title) {
        Project project = Project.builder()
                .id(IdUtil.genProjectId())
                .title(title)
                .build();
        return projectRepository.create(project);
    }

    public Project updateProject(String projectId, String title) {
        Project project = Project.builder()
                .id(projectId)
                .title(title)
                .build();
        synchronized (projectWriteLock.lockFor(projectId)) {
            return projectRepository.update(project);
        }
    }

    public void deleteProject(String projectId) {
        synchronized (projectWriteLock.lockFor(projectId)) {
            projectRepository.delete(projectId).forEach(eventPublisher::close);
        }
    }

    public void saveTheme(Project project) {
        String projectId = project.getId();
        String themeId = project.getTheme();
        synchronized (projectWriteLock.lockFor(projectId)) {
            Map<String, SseEvent.Changed> changes = transaction.execute(status -> {
                projectRepository.requireWritable(projectId);
                validateTheme(themeId);
                return projectRepository.updateTheme(projectId, themeId)
                        ? projectRepository.invalidateCheckpoints(projectId) : Map.of();
            });
            changes.forEach(eventPublisher::publish);
        }
    }

    public void savePositions(Project project) {
        String projectId = project.getId();
        synchronized (projectWriteLock.lockFor(projectId)) {
            Map<String, SseEvent.Changed> changes = transaction.execute(status -> {
                projectRepository.requireWritable(projectId);
                return projectRepository.updatePositions(project)
                        ? projectRepository.invalidateCheckpoints(projectId) : Map.of();
            });
            changes.forEach(eventPublisher::publish);
        }
    }

    public void saveLogo(String projectId, byte[] bytes) {
        synchronized (projectWriteLock.lockFor(projectId)) {
            Map<String, SseEvent.Changed> changes = transaction.execute(status -> {
                projectRepository.requireWritable(projectId);
                return projectRepository.updateLogo(projectId, bytes)
                        ? projectRepository.invalidateCheckpoints(projectId) : Map.of();
            });
            changes.forEach(eventPublisher::publish);
        }
    }

    public void deleteLogo(String projectId) {
        synchronized (projectWriteLock.lockFor(projectId)) {
            Map<String, SseEvent.Changed> changes = transaction.execute(status -> {
                projectRepository.requireWritable(projectId);
                return projectRepository.updateLogo(projectId, null)
                        ? projectRepository.invalidateCheckpoints(projectId) : Map.of();
            });
            changes.forEach(eventPublisher::publish);
        }
    }

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

    private void validateTheme(String themeId) {
        Asset theme = assetRepository.findAssetById(themeId);
        if (theme == null) {
            throw notFound(ErrorCode.ASSET_NOT_FOUND);
        }
        if (theme.getType() != AssetType.THEME || !theme.getEnabled() || theme.getContent().isBlank()) {
            throw invalid("需选择已启用且正文有效的主题");
        }
    }
}
