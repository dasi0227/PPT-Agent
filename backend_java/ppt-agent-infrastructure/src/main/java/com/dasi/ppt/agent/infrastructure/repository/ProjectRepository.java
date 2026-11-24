package com.dasi.ppt.agent.infrastructure.repository;

import com.dasi.ppt.agent.domain.project.model.entity.Project;
import com.dasi.ppt.agent.domain.project.model.enumeration.DecorationPosition;
import com.dasi.ppt.agent.domain.project.repository.IProjectRepository;
import com.dasi.ppt.agent.domain.shared.exception.ErrorCode;
import com.dasi.ppt.agent.domain.shared.sse.ChangeArea;
import com.dasi.ppt.agent.domain.shared.sse.SseEvent;
import com.dasi.ppt.agent.infrastructure.persistence.database.mapper.CheckpointMapper;
import com.dasi.ppt.agent.infrastructure.persistence.database.mapper.ProjectMapper;
import com.dasi.ppt.agent.infrastructure.persistence.database.mapper.ReceiptMapper;
import com.dasi.ppt.agent.infrastructure.persistence.database.mapper.SessionMapper;
import com.dasi.ppt.agent.infrastructure.persistence.database.po.ProjectPo;
import com.dasi.ppt.agent.infrastructure.persistence.database.po.SessionPo;
import com.dasi.ppt.agent.infrastructure.persistence.filesystem.FileStorer;
import com.dasi.ppt.agent.infrastructure.persistence.filesystem.PathResolver;
import jakarta.annotation.PostConstruct;
import lombok.RequiredArgsConstructor;
import org.springframework.dao.DataAccessException;
import org.springframework.stereotype.Repository;
import org.springframework.transaction.annotation.Transactional;

import java.util.HashMap;
import java.util.HashSet;
import java.util.List;
import java.util.Map;
import java.util.Set;

import static com.dasi.ppt.agent.domain.shared.exception.BusinessException.notFound;

@Repository
@RequiredArgsConstructor
public class ProjectRepository implements IProjectRepository {

    private final ProjectMapper projectMapper;
    private final CheckpointMapper checkpointMapper;
    private final SessionMapper sessionMapper;
    private final ReceiptMapper receiptMapper;
    private final FileStorer fileStorer;
    private final PathResolver pathResolver;

    @PostConstruct
    void recoverDirectories() {
        fileStorer.recoverProjectDirectories(pathResolver, projectMapper::exists);
    }

    @Override
    public void requireWritable(String projectId) {
        fileStorer.requireWritable(pathResolver.projectDirectory(projectId));
    }

    @Override
    public List<Project> listProjects() {
        return projectMapper.list()
                .stream()
                .map(this::toProject)
                .toList();
    }

    private Project toProject(ProjectPo row) {
        return Project.builder()
                .id(row.getId())
                .title(row.getTitle())
                .updatedAt(row.getUpdatedAt())
                .build();
    }

    @Override
    public Project findAppearance(String projectId) {
        ProjectPo row = projectMapper.findAppearance(projectId);
        if (row == null) {
            throw notFound(ErrorCode.PROJECT_NOT_FOUND);
        }
        return Project.builder()
                .id(row.getId())
                .theme(row.getTheme())
                .slidePaginationPosition(DecorationPosition.fromValue(row.getSlidePaginationPosition()))
                .deckTitlePosition(DecorationPosition.fromValue(row.getDeckTitlePosition()))
                .sectionTitlePosition(DecorationPosition.fromValue(row.getSectionTitlePosition()))
                .logoPosition(DecorationPosition.fromValue(row.getProjectLogoPosition()))
                .logoPresent(row.isLogoPresent())
                .build();
    }

    @Override
    public byte[] findLogo(String projectId) {
        ProjectPo row = projectMapper.findLogo(projectId);
        return row == null ? null : row.getLogo();
    }

    @Override
    public Project create(Project project) {
        ProjectPo row = projectMapper.insert(ProjectPo.builder()
                .id(project.getId())
                .title(project.getTitle())
                .updatedAt(System.currentTimeMillis())
                .build());
        fileStorer.createProjectDirectory(pathResolver.projectDirectory(row.getId()));
        return toProject(row);
    }

    @Override
    public Project updateTitle(Project project) {
        ProjectPo row = projectMapper.updateTitle(ProjectPo.builder()
                .id(project.getId())
                .title(project.getTitle())
                .updatedAt(System.currentTimeMillis())
                .build());
        if (row == null) {
            throw notFound(ErrorCode.PROJECT_NOT_FOUND);
        }
        return toProject(row);
    }

    @Override
    @Transactional
    public List<String> delete(String projectId) {
        List<String> sessions = sessionMapper.listRevisions(projectId)
                .stream()
                .map(SessionPo::getId)
                .toList();
        receiptMapper.deleteByProject(projectId);
        if (projectMapper.delete(projectId) == 0) {
            throw notFound(ErrorCode.PROJECT_NOT_FOUND);
        }
        fileStorer.stageProjectDeletion(pathResolver.projectDirectory(projectId));
        return sessions;
    }

    @Override
    public boolean updateTheme(String projectId, String themeId) {
        int updated = projectMapper.updateTheme(projectId, themeId, System.currentTimeMillis());
        return appearanceChanged(projectId, updated);
    }

    @Override
    public boolean updatePositions(Project project) {
        int updated = projectMapper.updatePositions(ProjectPo.builder()
                .id(project.getId())
                .slidePaginationPosition(project.getSlidePaginationPosition().getValue())
                .deckTitlePosition(project.getDeckTitlePosition().getValue())
                .sectionTitlePosition(project.getSectionTitlePosition().getValue())
                .projectLogoPosition(project.getLogoPosition().getValue())
                .updatedAt(System.currentTimeMillis())
                .build());
        return appearanceChanged(project.getId(), updated);
    }

    @Override
    public boolean updateLogo(String projectId, byte[] logo) {
        int updated = projectMapper.updateLogo(projectId, logo, System.currentTimeMillis());
        return appearanceChanged(projectId, updated);
    }

    private boolean appearanceChanged(String projectId, int updated) {
        if (updated != 0) {
            return true;
        }
        if (!projectMapper.exists(projectId)) {
            throw notFound(ErrorCode.PROJECT_NOT_FOUND);
        }
        return false;
    }

    @Override
    public Map<String, SseEvent.Changed> invalidateCheckpoints(String projectId) {
        Set<String> affected = new HashSet<>(sessionMapper.advanceCheckpointRevisions(projectId));
        checkpointMapper.deleteByProject(projectId);
        Map<String, SseEvent.Changed> changes = new HashMap<>();
        for (SessionPo session : sessionMapper.listRevisions(projectId)) {
            List<ChangeArea> areas = affected.contains(session.getId())
                    ? List.of(ChangeArea.CONTENT, ChangeArea.CHECKPOINTS) : List.of(ChangeArea.CONTENT);
            changes.put(session.getId(), SseEvent.Changed.builder()
                    .revision(Long.toString(session.getRevision()))
                    .areas(areas)
                    .build());
        }
        return changes;
    }
}
