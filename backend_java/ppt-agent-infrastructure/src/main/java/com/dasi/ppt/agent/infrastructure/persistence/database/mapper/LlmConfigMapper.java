package com.dasi.ppt.agent.infrastructure.persistence.database.mapper;

import com.dasi.ppt.agent.infrastructure.persistence.database.po.LlmConfigPo;
import org.apache.ibatis.annotations.Param;

import java.util.List;

public interface LlmConfigMapper {

    List<LlmConfigPo> listModels();

    LlmConfigPo findModelById(String id);

    boolean existsConflictingModelName(@Param("name") String name, @Param("modelId") String modelId);

    boolean existsModelById(@Param("id") String id);

    boolean isLastModel(String id);

    void insertModel(LlmConfigPo model);

    int updateModel(LlmConfigPo model);

    int deleteModel(@Param("id") String id);
}
