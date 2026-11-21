package com.dasi.ppt.agent.infrastructure.repository;

import com.dasi.ppt.agent.domain.project.model.entity.Project;
import com.dasi.ppt.agent.domain.project.repository.IProjectRepository;
import com.dasi.ppt.agent.infrastructure.persistence.database.mapper.AssetMapper;
import com.dasi.ppt.agent.infrastructure.persistence.database.mapper.ProjectMapper;
import com.dasi.ppt.agent.infrastructure.persistence.filesystem.FileStorer;
import com.dasi.ppt.agent.infrastructure.persistence.filesystem.PathResolver;
import lombok.RequiredArgsConstructor;
import org.springframework.dao.DataAccessException;
import org.springframework.stereotype.Repository;

import java.util.List;

@Repository
@RequiredArgsConstructor
public class ProjectRepository implements IProjectRepository {

    private final ProjectMapper projectMapper;
    private final AssetMapper assetMapper;
    private final FileStorer fileStorer;
    private final PathResolver pathResolver;

    @Override
    public List<Project> listProjects() {
        throw new UnsupportedOperationException();
    }

    @Override
    public boolean isDatabaseAvailable() {
        try {
            return projectMapper.hasApplicationSchema();
        } catch (DataAccessException unavailable) {
            return false;
        }
    }
}
