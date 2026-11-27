package com.dasi.ppt.agent.domain.project.service;

import com.dasi.ppt.agent.domain.asset.model.entity.Asset;
import com.dasi.ppt.agent.domain.asset.model.enumeration.AssetType;
import com.dasi.ppt.agent.domain.asset.repository.IAssetRepository;
import com.dasi.ppt.agent.domain.project.model.entity.Project;
import com.dasi.ppt.agent.domain.project.model.enumeration.DecorationPosition;
import com.dasi.ppt.agent.domain.project.repository.IProjectRepository;
import com.dasi.ppt.agent.domain.shared.exception.ErrorCode;
import com.dasi.ppt.agent.domain.shared.service.NotifyService;
import com.dasi.ppt.agent.domain.shared.service.LockService;
import com.dasi.ppt.agent.domain.shared.sse.EventPublisher;
import com.dasi.ppt.agent.domain.shared.sse.ProjectChangeArea;
import com.dasi.ppt.agent.domain.shared.util.IdUtil;
import lombok.RequiredArgsConstructor;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;
import org.springframework.transaction.support.TransactionTemplate;

import java.util.EnumSet;
import java.util.List;
import java.util.function.IntSupplier;

import static com.dasi.ppt.agent.domain.shared.exception.BusinessException.*;

@Service
@RequiredArgsConstructor
public class ProjectService {

    private final IProjectRepository projectRepository;
    private final IAssetRepository assetRepository;
    private final TransactionTemplate transaction;
    private final LockService lockService;
    private final EventPublisher eventPublisher;
    private final NotifyService notifyService;

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
        synchronized (lockService.projectLock(projectId)) {
            if (projectRepository.hasPendingFileOperation(projectId)) {
                throw conflict(ErrorCode.PROJECT_BUSY);
            }
            if (projectRepository.updateProjectTitle(projectId, title, System.currentTimeMillis()) == 0) {
                throw notFound(ErrorCode.PROJECT_NOT_FOUND);
            }
        }
    }

    public void deleteProject(String projectId) {
        // 1. 上锁
        synchronized (lockService.projectLock(projectId)) {
            List<String> deletedSessionIds = transaction.execute(status -> {
                // 2. 判断是否有正在执行的操作
                if (projectRepository.hasPendingFileOperation(projectId)) {
                    throw conflict(ErrorCode.PROJECT_BUSY);
                }
                // 3. 拿到所有会话 id
                List<String> sessionIds = projectRepository.listProjectSessionIds(projectId);
                // 4. 删除项目
                if (projectRepository.deleteProject(projectId) == 0) {
                    throw notFound(ErrorCode.PROJECT_NOT_FOUND);
                }
                return sessionIds;
            });
            // 5. 关闭所有会话的连接
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

    public byte[] findProjectLogo(String projectId) {
        return projectRepository.findProjectLogo(projectId);
    }

    public void saveProjectTheme(Project project) {
        updateProjectAppearance(project.getId(), () -> {
            Asset theme = assetRepository.findAssetById(project.getTheme());
            if (theme == null) {
                throw notFound(ErrorCode.ASSET_NOT_FOUND);
            }
            if (theme.getType() != AssetType.THEME || !theme.getEnabled() || theme.getContent().isBlank()) {
                throw invalid("需选择已启用且正文有效的主题");
            }
            project.setUpdatedAt(System.currentTimeMillis());
            return projectRepository.updateProjectTheme(project);
        });
    }

    public void saveProjectPositions(Project project) {
        updateProjectAppearance(project.getId(), () -> {
            EnumSet<DecorationPosition> occupied = EnumSet.noneOf(DecorationPosition.class);
            for (DecorationPosition position : List.of(
                    project.getSlidePaginationPosition(),
                    project.getDeckTitlePosition(),
                    project.getChapterTitlePosition(),
                    project.getLogoPosition())) {
                if (position != DecorationPosition.NONE && !occupied.add(position)) {
                    throw invalid("显示的装饰位置不能重叠");
                }
            }
            project.setUpdatedAt(System.currentTimeMillis());
            return projectRepository.updateProjectPositions(project);
        });
    }

    // logo 为 null 时清除项目 Logo。
    public void saveProjectLogo(Project project) {
        updateProjectAppearance(project.getId(), () -> {
            project.setUpdatedAt(System.currentTimeMillis());
            return projectRepository.updateProjectLogo(project);
        });
    }

    private void updateProjectAppearance(String projectId, IntSupplier update) {
        synchronized (lockService.projectLock(projectId)) {
            List<String> sessionIds = transaction.execute(status -> {
                if (projectRepository.hasPendingFileOperation(projectId)) {
                    throw conflict(ErrorCode.PROJECT_BUSY);
                }
                if (!projectRepository.existsProject(projectId)) {
                    throw notFound(ErrorCode.PROJECT_NOT_FOUND);
                }
                int updatedRows = update.getAsInt();
                return updatedRows == 0 ? List.of() : projectRepository.listProjectSessionIds(projectId);
            });
            notifyService.notifyProjectChanged(sessionIds, ProjectChangeArea.APPEARANCE);
        }
    }
}
