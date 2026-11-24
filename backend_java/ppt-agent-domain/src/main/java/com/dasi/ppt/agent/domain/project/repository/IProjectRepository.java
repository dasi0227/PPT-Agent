package com.dasi.ppt.agent.domain.project.repository;

import com.dasi.ppt.agent.domain.project.model.entity.Project;
import com.dasi.ppt.agent.domain.shared.sse.SseEvent;

import java.util.List;
import java.util.Map;

public interface IProjectRepository {

    List<Project> listProjects();

    void requireWritable(String projectId);

    Project findAppearance(String projectId);

    byte[] findLogo(String projectId);

    void create(Project project);

    void updateTitle(Project project);

    List<String> delete(String projectId);

    void updateAppearance(Project project);

    boolean updateLogo(String projectId, byte[] logo);

    Map<String, SseEvent.Changed> invalidateCheckpoints(String projectId);

}
