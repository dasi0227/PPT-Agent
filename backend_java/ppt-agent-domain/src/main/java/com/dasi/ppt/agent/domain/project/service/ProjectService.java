package com.dasi.ppt.agent.domain.project.service;

import com.dasi.ppt.agent.domain.asset.model.entity.Asset;
import com.dasi.ppt.agent.domain.asset.model.enumeration.AssetType;
import com.dasi.ppt.agent.domain.asset.repository.IAssetRepository;
import com.dasi.ppt.agent.domain.project.model.entity.Project;
import com.dasi.ppt.agent.domain.project.model.enumeration.DecorationPosition;
import com.dasi.ppt.agent.domain.project.repository.IProjectRepository;
import com.dasi.ppt.agent.domain.setting.model.entity.FileOpenSettings;
import com.dasi.ppt.agent.domain.setting.service.SettingService;
import com.dasi.ppt.agent.domain.shared.exception.ErrorCode;
import com.dasi.ppt.agent.domain.shared.gateway.IPlatformGateway;
import com.dasi.ppt.agent.domain.shared.sse.EventPublisher;
import com.dasi.ppt.agent.domain.shared.sse.ProjectChangeArea;
import com.dasi.ppt.agent.domain.shared.sse.SseEvent;
import com.dasi.ppt.agent.domain.shared.util.IdUtil;
import lombok.RequiredArgsConstructor;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;
import org.springframework.transaction.support.TransactionTemplate;

import java.nio.file.Path;
import java.util.EnumSet;
import java.util.List;
import java.util.Locale;

import static com.dasi.ppt.agent.domain.shared.constant.SystemConstant.SYSTEM_OPEN;
import static com.dasi.ppt.agent.domain.shared.constant.SystemConstant.INHERIT_OPEN;
import static com.dasi.ppt.agent.domain.shared.exception.BusinessException.conflict;
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

    @Transactional
    public String createProject(Project project) {
        project.setId(IdUtil.genProjectId());
        project.setUpdatedAt(System.currentTimeMillis());
        projectRepository.insertProject(project);
        return project.getId();
    }

    public void updateProject(String projectId, String title) {
        synchronized (projectWriteLock.lockFor(projectId)) {
            if (projectRepository.hasPendingFileOperation(projectId)) {
                throw conflict(ErrorCode.PROJECT_BUSY);
            }
            if (projectRepository.updateProjectTitle(projectId, title, System.currentTimeMillis()) == 0) {
                throw notFound(ErrorCode.PROJECT_NOT_FOUND);
            }
        }
    }

    // 1.上锁 2.判断是否可更改 3.拿到所有会话id 4.删除整个项目 5.关闭所有会话的连接
    public void deleteProject(String projectId) {
        synchronized (projectWriteLock.lockFor(projectId)) {
            List<String> deletedSessionIds = transaction.execute(status -> {
                if (projectRepository.hasPendingFileOperation(projectId)) {
                    throw conflict(ErrorCode.PROJECT_BUSY);
                }
                List<String> sessionIds = projectRepository.listProjectSessionIds(projectId);
                if (projectRepository.deleteProject(projectId) == 0) {
                    throw notFound(ErrorCode.PROJECT_NOT_FOUND);
                }
                return sessionIds;
            });
            deletedSessionIds.forEach(eventPublisher::close);
        }
    }

    public Project getProjectAppearance(String projectId) {
        Project project = projectRepository.findProjectAppearance(projectId);
        if (project == null) {
            throw notFound(ErrorCode.PROJECT_NOT_FOUND);
        }
        return project;
    }

    // 1.上锁 2.判断是否可更改 3.检查主题可用 4.拿到所有会话id 5.通知项目已经改变
    public void saveProjectTheme(String projectId, String themeId) {
        synchronized (projectWriteLock.lockFor(projectId)) {
            List<String> sessionIds = transaction.execute(status -> {
                if (projectRepository.hasPendingFileOperation(projectId)) {
                    throw conflict(ErrorCode.PROJECT_BUSY);
                }
                if (!projectRepository.existsProject(projectId)) {
                    throw notFound(ErrorCode.PROJECT_NOT_FOUND);
                }

                Asset theme = assetRepository.findAssetById(themeId);
                if (theme == null) {
                    throw notFound(ErrorCode.ASSET_NOT_FOUND);
                }
                if (theme.getType() != AssetType.THEME || !theme.getEnabled() || theme.getContent().isBlank()) {
                    throw invalid("需选择已启用且正文有效的主题");
                }

                int updatedRows = projectRepository.updateProjectTheme(projectId, themeId, System.currentTimeMillis());
                return updatedRows == 0 ? List.of() : projectRepository.listProjectSessionIds(projectId);
            });
            publishAppearanceChange(sessionIds);
        }
    }

    public void saveProjectPositions(Project project) {
        String projectId = project.getId();
        synchronized (projectWriteLock.lockFor(projectId)) {
            List<String> sessionIds = transaction.execute(status -> {
                if (projectRepository.hasPendingFileOperation(projectId)) {
                    throw conflict(ErrorCode.PROJECT_BUSY);
                }
                if (!projectRepository.existsProject(projectId)) {
                    throw notFound(ErrorCode.PROJECT_NOT_FOUND);
                }

                EnumSet<DecorationPosition> occupied = EnumSet.noneOf(DecorationPosition.class);
                for (DecorationPosition position : List.of(project.getSlidePaginationPosition(), project.getDeckTitlePosition(), project.getChapterTitlePosition(), project.getLogoPosition())) {
                    if (position != DecorationPosition.NONE && !occupied.add(position)) {
                        throw invalid("显示的装饰位置不能重叠");
                    }
                }
                project.setUpdatedAt(System.currentTimeMillis());

                int updatedRows = projectRepository.updateProjectPositions(project);
                return updatedRows == 0 ? List.of() : projectRepository.listProjectSessionIds(projectId);
            });
            publishAppearanceChange(sessionIds);
        }
    }

    public byte[] findProjectLogo(String projectId) {
        return projectRepository.findProjectLogo(projectId);
    }

    public void saveProjectLogo(String projectId, byte[] logo) {
        synchronized (projectWriteLock.lockFor(projectId)) {
            List<String> sessionIds = transaction.execute(status -> {
                if (projectRepository.hasPendingFileOperation(projectId)) {
                    throw conflict(ErrorCode.PROJECT_BUSY);
                }
                if (!projectRepository.existsProject(projectId)) {
                    throw notFound(ErrorCode.PROJECT_NOT_FOUND);
                }
                int updatedRows = projectRepository.updateProjectLogo(projectId, logo, System.currentTimeMillis());
                return updatedRows == 0 ? List.of() : projectRepository.listProjectSessionIds(projectId);
            });
            publishAppearanceChange(sessionIds);
        }
    }

    public void deleteProjectLogo(String projectId) {
        synchronized (projectWriteLock.lockFor(projectId)) {
            List<String> sessionIds = transaction.execute(status -> {
                if (projectRepository.hasPendingFileOperation(projectId)) {
                    throw conflict(ErrorCode.PROJECT_BUSY);
                }
                if (!projectRepository.existsProject(projectId)) {
                    throw notFound(ErrorCode.PROJECT_NOT_FOUND);
                }
                int updatedRows = projectRepository.updateProjectLogo(projectId, null, System.currentTimeMillis());
                return updatedRows == 0 ? List.of() : projectRepository.listProjectSessionIds(projectId);
            });
            publishAppearanceChange(sessionIds);
        }
    }

    private void publishAppearanceChange(List<String> sessionIds) {
        SseEvent.ProjectChanged event = SseEvent.ProjectChanged.builder()
                .areas(List.of(ProjectChangeArea.APPEARANCE))
                .build();
        sessionIds.forEach(sessionId -> eventPublisher.publish(sessionId, event));
    }

    public void openFile(String projectId, String relativePath) {
        FileOpenSettings settings = settingService.getFileOpenSettings();
        String filename = relativePath.toLowerCase(Locale.ROOT);
        String choice = settings.getDefaultOpen();
        if (filename.endsWith(".json")) {
            choice = settings.getJsonOpen();
        } else if (filename.endsWith(".html")) {
            choice = settings.getHtmlOpen();
        }
        if (INHERIT_OPEN.equals(choice)) {
            choice = settings.getDefaultOpen();
        }
        Path application = SYSTEM_OPEN.equals(choice) ? null : Path.of(choice);
        platformGateway.openFile(projectId, relativePath, application);
    }
}
