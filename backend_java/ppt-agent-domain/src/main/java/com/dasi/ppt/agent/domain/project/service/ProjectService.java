package com.dasi.ppt.agent.domain.project.service;

import com.dasi.ppt.agent.domain.asset.model.entity.Asset;
import com.dasi.ppt.agent.domain.asset.model.enumeration.AssetType;
import com.dasi.ppt.agent.domain.asset.repository.IAssetRepository;
import com.dasi.ppt.agent.domain.shared.gateway.IPlatformGateway;
import com.dasi.ppt.agent.domain.project.model.entity.Project;
import com.dasi.ppt.agent.domain.project.repository.IProjectRepository;
import com.dasi.ppt.agent.domain.setting.model.entity.FileOpenSettings;
import com.dasi.ppt.agent.domain.setting.service.SettingService;
import com.dasi.ppt.agent.domain.shared.sse.EventPublisher;
import com.dasi.ppt.agent.domain.shared.sse.SseEvent;
import com.dasi.ppt.agent.domain.shared.util.IdUtil;
import lombok.RequiredArgsConstructor;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;
import org.springframework.transaction.support.TransactionTemplate;

import javax.imageio.ImageIO;
import javax.imageio.ImageReader;
import javax.imageio.stream.MemoryCacheImageInputStream;
import java.io.ByteArrayInputStream;
import java.io.IOException;
import java.nio.file.Path;
import java.util.List;
import java.util.Locale;
import java.util.Map;

import static com.dasi.ppt.agent.domain.setting.model.entity.FileOpenSettings.INHERIT_OPEN;
import static com.dasi.ppt.agent.domain.setting.model.entity.FileOpenSettings.SYSTEM_OPEN;
import static com.dasi.ppt.agent.domain.shared.exception.BusinessException.invalid;

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

    private static final int MAX_LOGO_BYTES = 2_000_000;

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
        project.normalize();
        return projectRepository.create(project);
    }

    public Project updateProject(String projectId, String title) {
        Project project = Project.builder()
                .id(projectId)
                .title(title)
                .build();
        project.normalize();
        synchronized (projectWriteLock.lockFor(projectId)) {
            projectRepository.requireWritable(projectId);
            return projectRepository.updateTitle(project);
        }
    }

    public void deleteProject(String projectId) {
        synchronized (projectWriteLock.lockFor(projectId)) {
            projectRepository.requireWritable(projectId);
            projectRepository.delete(projectId).forEach(eventPublisher::close);
        }
    }

    public void saveTheme(Project project) {
        String projectId = project.getId();
        String themeId = project.getTheme();
        synchronized (projectWriteLock.lockFor(projectId)) {
            projectRepository.requireWritable(projectId);
            Map<String, SseEvent.Changed> changes = transaction.execute(status -> {
                validateTheme(themeId);
                return projectRepository.updateTheme(projectId, themeId)
                        ? projectRepository.invalidateCheckpoints(projectId) : Map.of();
            });
            changes.forEach(eventPublisher::publish);
        }
    }

    public void savePositions(Project project) {
        project.validatePositions();
        String projectId = project.getId();
        synchronized (projectWriteLock.lockFor(projectId)) {
            projectRepository.requireWritable(projectId);
            Map<String, SseEvent.Changed> changes = transaction.execute(status ->
                    projectRepository.updatePositions(project)
                    ? projectRepository.invalidateCheckpoints(projectId) : Map.of());
            changes.forEach(eventPublisher::publish);
        }
    }

    public void saveLogo(String projectId, byte[] bytes) {
        validateLogo(bytes);
        synchronized (projectWriteLock.lockFor(projectId)) {
            projectRepository.requireWritable(projectId);
            Map<String, SseEvent.Changed> changes = transaction.execute(status ->
                    projectRepository.updateLogo(projectId, bytes)
                    ? projectRepository.invalidateCheckpoints(projectId) : Map.of());
            changes.forEach(eventPublisher::publish);
        }
    }

    public void deleteLogo(String projectId) {
        synchronized (projectWriteLock.lockFor(projectId)) {
            projectRepository.requireWritable(projectId);
            Map<String, SseEvent.Changed> changes = transaction.execute(status ->
                    projectRepository.updateLogo(projectId, null)
                    ? projectRepository.invalidateCheckpoints(projectId) : Map.of());
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
        Asset theme = assetRepository.findAsset(themeId);
        if (theme.getType() != AssetType.THEME || !theme.getEnabled() || theme.getContent().isBlank()) {
            throw invalid("需选择已启用且正文有效的主题");
        }
    }

    private void validateLogo(byte[] bytes) {
        if (bytes.length > MAX_LOGO_BYTES) {
            throw invalid("Logo 不能超过 2 MB");
        }
        ImageReader reader = ImageIO.getImageReadersByFormatName("PNG").next();
        try (MemoryCacheImageInputStream input = new MemoryCacheImageInputStream(new ByteArrayInputStream(bytes))) {
            reader.setInput(input);
            reader.getWidth(0);
            reader.getHeight(0);
        } catch (IOException | IllegalArgumentException ex) {
            throw invalid("Logo 需为有效的 PNG 图片");
        } finally {
            reader.dispose();
        }
    }
}
