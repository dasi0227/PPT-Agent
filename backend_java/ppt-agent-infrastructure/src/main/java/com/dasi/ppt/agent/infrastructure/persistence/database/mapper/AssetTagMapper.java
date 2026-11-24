package com.dasi.ppt.agent.infrastructure.persistence.database.mapper;

import org.apache.ibatis.annotations.Param;

import java.util.List;

public interface AssetTagMapper {

    void insert(@Param("assetId") String assetId, @Param("tagIds") List<String> tagIds);

    void deleteByAsset(String assetId);
}
