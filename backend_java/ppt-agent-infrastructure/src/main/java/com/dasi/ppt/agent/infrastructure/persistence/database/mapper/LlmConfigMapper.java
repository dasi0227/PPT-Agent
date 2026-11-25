package com.dasi.ppt.agent.infrastructure.persistence.database.mapper;

import com.dasi.ppt.agent.infrastructure.persistence.database.po.LlmConfigPo;
import org.apache.ibatis.annotations.Param;

import java.util.List;

public interface LlmConfigMapper {

    List<LlmConfigPo> list();

    LlmConfigPo find(String id);

    boolean nameExists(@Param("name") String name, @Param("excludeId") String excludeId);

    boolean exists(@Param("id") String id);

    int count();

    void insert(LlmConfigPo model);

    int update(LlmConfigPo model);

    void delete(@Param("id") String id);
}
