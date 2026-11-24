package com.dasi.ppt.agent.infrastructure.persistence.database.mapper;

import com.dasi.ppt.agent.infrastructure.persistence.database.po.ProjectPo;
import org.apache.ibatis.annotations.Param;

import java.util.List;

public interface ProjectMapper {

    List<ProjectPo> list();

    boolean exists(String id);

    ProjectPo findAppearance(String id);

    ProjectPo findLogo(String id);

    void insert(ProjectPo project);

    int updateTitle(ProjectPo project);

    void updateAppearance(ProjectPo project);

    int updateLogo(@Param("id") String id, @Param("logo") byte[] logo, @Param("updatedAt") long updatedAt);

    int delete(String id);
}
