package com.dasi.ppt.agent.infrastructure.persistence.database.mapper;

import org.apache.ibatis.annotations.Param;

import java.util.List;

public interface AssetTagMapper {

    List<String> listIds(String assetId);

    int insert(@Param("assetId") String assetId, @Param("tagIds") List<String> tagIds);

    int deleteExcept(@Param("assetId") String assetId, @Param("tagIds") List<String> tagIds);
}
