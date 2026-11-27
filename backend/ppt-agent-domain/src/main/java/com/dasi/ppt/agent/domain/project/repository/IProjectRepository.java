package com.dasi.ppt.agent.domain.project.repository;

import com.dasi.ppt.agent.domain.project.model.entity.FileChange;
import com.dasi.ppt.agent.domain.project.model.entity.Project;

import java.util.List;

public interface IProjectRepository {

    List<Project> listProjects();

    void insertProject(Project project);

    int updateProjectTitle(String projectId, String title, long updatedAt);

    List<String> listProjectSessionIds(String projectId);

    int deleteProject(String projectId);

    boolean hasPendingFileOperation(String projectId);

    boolean isProjectBusy(String projectId);

    boolean existsProject(String projectId);

    Project findProjectAppearance(String projectId);

    int updateProjectTheme(Project project);

    int updateProjectPositions(Project project);

    byte[] findProjectLogo(String projectId);

    int updateProjectLogo(Project project);

    void commitFiles(String projectId, List<FileChange> fileChanges);
}
