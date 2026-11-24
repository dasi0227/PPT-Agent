package com.dasi.ppt.agent.domain.project.service;

import com.dasi.ppt.agent.domain.asset.model.entity.Asset;
import com.dasi.ppt.agent.domain.asset.model.enumeration.AssetType;
import com.dasi.ppt.agent.domain.asset.repository.IAssetRepository;
import com.dasi.ppt.agent.domain.gateway.IPlatformGateway;
import com.dasi.ppt.agent.domain.project.model.entity.ProjectLogo;
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

import java.nio.file.Path;
import java.util.List;
import java.util.Locale;
import java.util.Map;
import java.util.function.BooleanSupplier;

import static com.dasi.ppt.agent.domain.setting.model.entity.FileOpenSettings.INHERIT_OPEN;
import static com.dasi.ppt.agent.domain.setting.model.entity.FileOpenSettings.SYSTEM_OPEN;
import static com.dasi.ppt.agent.domain.shared.exception.BusinessException.invalid;

@Service
@RequiredArgsConstructor
public class ProjectService {

    private static final String DEFAULT_THEME = "editorial-serif";

    private final IProjectRepository projectRepository;
    private final IAssetRepository assetRepository;
    private final TransactionTemplate transactions;
    private final ProjectWriteLock projectWriteLock;
    private final EventPublisher eventPublisher;
    private final SettingService settingService;
    private final IPlatformGateway platformGateway;

    public List<Project> listProjects() {
        return projectRepository.listProjects();
    }

    @Transactional
    public String createProject(String title) {
        Project project = Project.builder()
                .id(IdUtil.genProjectId())
                .title(title)
                .theme(DEFAULT_THEME)
                .build();
        project.normalizeTitle();
        validateTheme(DEFAULT_THEME);
        projectRepository.create(project);
        return project.getId();
    }

    public void updateProject(String projectId, String title) {
        Project project = Project.builder()
                .id(projectId)
                .title(title)
                .build();
        project.normalizeTitle();
        writeProject(projectId, () -> projectRepository.updateTitle(project));
    }

    public void deleteProject(String projectId) {
        writeProject(projectId, () -> projectRepository.delete(projectId).forEach(eventPublisher::close));
    }

    public Project findAppearance(String projectId) {
        return projectRepository.findAppearance(projectId);
    }

    public void saveAppearance(Project submitted) {
        submitted.validateAppearance();
        changeAppearance(submitted.getId(), () -> {
            Project current = findAppearance(submitted.getId());
            boolean themeChanged = submitted.getTheme() != null && !submitted.getTheme().equals(current.getTheme());
            boolean positionsChanged = submitted.getPositions() != null && !submitted.getPositions().equals(current.getPositions());
            if (!themeChanged && !positionsChanged) {
                return false;
            }
            if (themeChanged) {
                validateTheme(submitted.getTheme());
                current.setTheme(submitted.getTheme());
            }
            if (positionsChanged) {
                current.setPositions(submitted.getPositions());
            }
            projectRepository.updateAppearance(current);
            return true;
        });
    }

    private void validateTheme(String themeId) {
        Asset theme = assetRepository.findAsset(themeId);
        if (theme.getType() != AssetType.THEME || !theme.getEnabled() || theme.getContent().isBlank()) {
            throw invalid("需选择已启用且正文有效的主题");
        }
        theme.validate();
    }

    public byte[] findLogo(String projectId) {
        return projectRepository.findLogo(projectId);
    }

    public void saveLogo(String projectId, ProjectLogo logo) {
        logo.validate();
        changeAppearance(projectId, () -> projectRepository.updateLogo(projectId, logo.getBytes()));
    }

    public void deleteLogo(String projectId) {
        changeAppearance(projectId, () -> projectRepository.updateLogo(projectId, null));
    }

    private void changeAppearance(String projectId, BooleanSupplier write) {
        writeProject(projectId, () -> {
            Map<String, SseEvent.Changed> changes = transactions.execute(status -> write.getAsBoolean()
                    ? projectRepository.invalidateCheckpoints(projectId) : Map.of());
            changes.forEach(eventPublisher::publish);
        });
    }

    private void writeProject(String projectId, Runnable action) {
        projectWriteLock.withLock(projectId, () -> {
            projectRepository.requireWritable(projectId);
            action.run();
        });
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
}
