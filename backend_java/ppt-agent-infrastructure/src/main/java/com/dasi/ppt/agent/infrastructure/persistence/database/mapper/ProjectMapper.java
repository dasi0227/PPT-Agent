package com.dasi.ppt.agent.infrastructure.persistence.database.mapper;

import com.dasi.ppt.agent.infrastructure.persistence.database.po.ProjectPo;
import org.apache.ibatis.annotations.Param;

import java.util.List;

public interface ProjectMapper {

    List<ProjectPo> listProjects();

    boolean existsProject(String projectId);

    ProjectPo findProjectAppearance(String projectId);

    ProjectPo findProjectLogo(String projectId);

    void insertProject(ProjectPo project);

    int updateProjectTitle(@Param("projectId") String projectId, @Param("title") String title, @Param("updatedAt") long updatedAt);

    int updateProjectTheme(@Param("projectId") String projectId, @Param("themeId") String themeId, @Param("updatedAt") long updatedAt);

    int updateProjectPositions(ProjectPo project);

    int updateProjectLogo(@Param("projectId") String projectId, @Param("logo") byte[] logo, @Param("updatedAt") long updatedAt);

    int deleteProject(String projectId);

    boolean hasActiveDocumentOperation(String projectId);

    void updateProjectTime(@Param("projectId") String projectId, @Param("updatedAt") long updatedAt);
}
