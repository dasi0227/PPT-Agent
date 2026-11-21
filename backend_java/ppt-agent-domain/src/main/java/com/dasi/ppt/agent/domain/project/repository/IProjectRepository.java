package com.dasi.ppt.agent.domain.project.repository;

import com.dasi.ppt.agent.domain.project.model.entity.Project;

import java.util.List;

public interface IProjectRepository {

    List<Project> listProjects();

    boolean isDatabaseAvailable();
}
