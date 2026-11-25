package com.dasi.ppt.agent.infrastructure.repository;

import com.dasi.ppt.agent.domain.project.model.entity.Project;
import com.dasi.ppt.agent.domain.project.model.enumeration.DecorationPosition;
import com.dasi.ppt.agent.domain.project.repository.IProjectRepository;
import com.dasi.ppt.agent.infrastructure.persistence.database.mapper.ProjectMapper;
import com.dasi.ppt.agent.infrastructure.persistence.database.mapper.ReceiptMapper;
import com.dasi.ppt.agent.infrastructure.persistence.database.mapper.SessionMapper;
import com.dasi.ppt.agent.infrastructure.persistence.database.po.ProjectPo;
import com.dasi.ppt.agent.infrastructure.persistence.filesystem.FileStorer;
import com.dasi.ppt.agent.infrastructure.persistence.filesystem.PathResolver;
import jakarta.annotation.PostConstruct;
import lombok.RequiredArgsConstructor;
import org.springframework.stereotype.Repository;

import java.util.List;

@Repository
@RequiredArgsConstructor
public class ProjectRepository implements IProjectRepository {

    private final ProjectMapper projectMapper;
    private final SessionMapper sessionMapper;
    private final ReceiptMapper receiptMapper;
    private final FileStorer fileStorer;
    private final PathResolver pathResolver;

    @PostConstruct
    void recoverDirectories() {
        fileStorer.recoverProjectDirectories();
    }

    @Override
    public List<Project> listProjects() {
        return projectMapper.listProjects()
                .stream()
                .map(ProjectRepository::toEntity)
                .toList();
    }

    @Override
    public void insertProject(Project project) {
        projectMapper.insertProject(toPo(project));
        fileStorer.createProjectDirectory(pathResolver.projectDirectory(project.getId()));
    }

    @Override
    public int updateProjectTitle(String projectId, String title, long updatedAt) {
        return projectMapper.updateProjectTitle(projectId, title, updatedAt);
    }

    @Override
    public List<String> listProjectSessionIds(String projectId) {
        return sessionMapper.listProjectSessionIds(projectId);
    }

    @Override
    public int deleteProject(String projectId) {
        receiptMapper.deleteByProject(projectId);
        int deleted = projectMapper.deleteProject(projectId);
        if (deleted > 0) {
            fileStorer.stageProjectDeletion(pathResolver.projectDirectory(projectId));
        }
        return deleted;
    }

    @Override
    public boolean hasPendingFileOperation(String projectId) {
        return fileStorer.hasPendingFileOperation(pathResolver.projectDirectory(projectId));
    }

    @Override
    public boolean existsProject(String projectId) {
        return projectMapper.existsProject(projectId);
    }

    @Override
    public Project findProjectAppearance(String projectId) {
        return toEntity(projectMapper.findProjectAppearance(projectId));
    }

    @Override
    public int updateProjectTheme(String projectId, String themeId, long updatedAt) {
        return projectMapper.updateProjectTheme(projectId, themeId, updatedAt);
    }

    @Override
    public int updateProjectPositions(Project project) {
        return projectMapper.updateProjectPositions(toPo(project));
    }

    @Override
    public byte[] findProjectLogo(String projectId) {
        ProjectPo po = projectMapper.findProjectLogo(projectId);
        return po == null ? null : po.getLogo();
    }

    @Override
    public int updateProjectLogo(String projectId, byte[] logo, long updatedAt) {
        return projectMapper.updateProjectLogo(projectId, logo, updatedAt);
    }

    private static Project toEntity(ProjectPo po) {
        if (po == null) return null;
        return Project.builder()
                .id(po.getId())
                .title(po.getTitle())
                .theme(po.getTheme())
                .slidePaginationPosition(position(po.getSlidePaginationPosition()))
                .deckTitlePosition(position(po.getDeckTitlePosition()))
                .chapterTitlePosition(position(po.getChapterTitlePosition()))
                .logoPosition(position(po.getProjectLogoPosition()))
                .logoPresent(po.isLogoPresent())
                .updatedAt(po.getUpdatedAt())
                .build();
    }

    private static ProjectPo toPo(Project project) {
        return ProjectPo.builder()
                .id(project.getId())
                .title(project.getTitle())
                .slidePaginationPosition(value(project.getSlidePaginationPosition()))
                .deckTitlePosition(value(project.getDeckTitlePosition()))
                .chapterTitlePosition(value(project.getChapterTitlePosition()))
                .projectLogoPosition(value(project.getLogoPosition()))
                .updatedAt(project.getUpdatedAt())
                .build();
    }

    private static DecorationPosition position(String value) {
        return value == null ? null : DecorationPosition.fromValue(value);
    }

    private static String value(DecorationPosition position) {
        return position == null ? null : position.getValue();
    }
}
