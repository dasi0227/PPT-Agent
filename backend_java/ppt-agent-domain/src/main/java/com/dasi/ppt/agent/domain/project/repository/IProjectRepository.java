package com.dasi.ppt.agent.domain.project.repository;

import com.dasi.ppt.agent.domain.project.model.entity.Project;
import com.dasi.ppt.agent.domain.shared.sse.SseEvent;

import java.util.List;
import java.util.Map;

public interface IProjectRepository {

    List<Project> listProjects();

    void insertProject(Project project);

    int updateProjectTitle(String projectId, String title, long updatedAt);

    List<String> listProjectSessionIds(String projectId);

    int deleteProject(String projectId);

    void requireWritableProjectFiles(String projectId);

    boolean existsProject(String projectId);

    Project findProjectAppearance(String projectId);

    int updateProjectTheme(String projectId, String themeId, long updatedAt);

    int updateProjectPositions(Project project);

    byte[] findProjectLogo(String projectId);

    int updateProjectLogo(String projectId, byte[] logo, long updatedAt);

    Map<String, SseEvent.Changed> invalidateCheckpoints(String projectId);
}
