package com.dasi.ppt.agent.infrastructure.repository;

import com.dasi.ppt.agent.domain.project.model.entity.Project;
import com.dasi.ppt.agent.domain.project.model.enumeration.DecorationPosition;
import com.dasi.ppt.agent.domain.project.model.enumeration.DecorationType;
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
                .map(row -> Project.builder()
                        .id(row.getId())
                        .title(row.getTitle())
                        .updatedAt(row.getUpdatedAt())
                        .build())
                .toList();
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
                .positions(Map.of(
                        DecorationType.SLIDE_PAGINATION, DecorationPosition.fromValue(row.getSlidePaginationPosition()),
                        DecorationType.DECK_TITLE, DecorationPosition.fromValue(row.getDeckTitlePosition()),
                        DecorationType.SECTION_TITLE, DecorationPosition.fromValue(row.getSectionTitlePosition()),
                        DecorationType.LOGO, DecorationPosition.fromValue(row.getProjectLogoPosition())))
                .logoPresent(row.isLogoPresent())
                .build();
    }

    @Override
    public byte[] findLogo(String projectId) {
        ProjectPo row = projectMapper.findLogo(projectId);
        return row == null ? null : row.getLogo();
    }

    @Override
    public void create(Project project) {
        projectMapper.insert(ProjectPo.builder()
                .id(project.getId())
                .title(project.getTitle())
                .theme(project.getTheme())
                .updatedAt(System.currentTimeMillis())
                .build());
        fileStorer.createProjectDirectory(pathResolver.projectDirectory(project.getId()));
    }

    @Override
    public void updateTitle(Project project) {
        int updated = projectMapper.updateTitle(ProjectPo.builder()
                .id(project.getId())
                .title(project.getTitle())
                .updatedAt(System.currentTimeMillis())
                .build());
        if (updated == 0) {
            throw notFound(ErrorCode.PROJECT_NOT_FOUND);
        }
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
    public void updateAppearance(Project project) {
        Map<DecorationType, DecorationPosition> positions = project.getPositions();
        projectMapper.updateAppearance(ProjectPo.builder()
                .id(project.getId())
                .theme(project.getTheme())
                .slidePaginationPosition(positions.get(DecorationType.SLIDE_PAGINATION).getValue())
                .deckTitlePosition(positions.get(DecorationType.DECK_TITLE).getValue())
                .sectionTitlePosition(positions.get(DecorationType.SECTION_TITLE).getValue())
                .projectLogoPosition(positions.get(DecorationType.LOGO).getValue())
                .updatedAt(System.currentTimeMillis())
                .build());
    }

    @Override
    public boolean updateLogo(String projectId, byte[] logo) {
        if (projectMapper.updateLogo(projectId, logo, System.currentTimeMillis()) != 0) {
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
