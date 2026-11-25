package com.dasi.ppt.agent.infrastructure.persistence.database.mapper;

import com.dasi.ppt.agent.infrastructure.persistence.database.po.ProjectPo;
import org.apache.ibatis.annotations.Param;

import java.util.List;

public interface ProjectMapper {

    List<ProjectPo> list();

    boolean exists(String id);

    ProjectPo findAppearance(String id);

    ProjectPo findLogo(String id);

    ProjectPo insert(ProjectPo project);

    ProjectPo updateTitle(@Param("id") String id, @Param("title") String title, @Param("updatedAt") long updatedAt);

    int updateTheme(@Param("id") String id, @Param("theme") String theme, @Param("updatedAt") long updatedAt);

    int updatePositions(ProjectPo project);

    int updateLogo(@Param("id") String id, @Param("logo") byte[] logo, @Param("updatedAt") long updatedAt);

    int delete(String id);
}
