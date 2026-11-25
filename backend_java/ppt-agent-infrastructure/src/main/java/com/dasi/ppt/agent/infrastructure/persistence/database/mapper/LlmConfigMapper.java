package com.dasi.ppt.agent.infrastructure.persistence.database.mapper;

import com.dasi.ppt.agent.infrastructure.persistence.database.po.LlmConfigPo;
import org.apache.ibatis.annotations.Param;

import java.util.List;

public interface LlmConfigMapper {

    List<LlmConfigPo> listModels();

    LlmConfigPo findModelById(String modelId);

    boolean existsConflictingModelName(@Param("name") String name, @Param("modelId") String modelId);

    boolean existsModelById(@Param("modelId") String modelId);

    boolean isLastModel(String modelId);

    void insertModel(LlmConfigPo model);

    int updateModel(LlmConfigPo model);

    int deleteModel(@Param("modelId") String modelId);
}
