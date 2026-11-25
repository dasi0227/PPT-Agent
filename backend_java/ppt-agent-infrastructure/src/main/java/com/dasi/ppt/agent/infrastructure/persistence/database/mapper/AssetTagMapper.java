package com.dasi.ppt.agent.infrastructure.persistence.database.mapper;

import org.apache.ibatis.annotations.Param;

import java.util.List;

public interface AssetTagMapper {

    int insertAssetTagLinks(@Param("assetId") String assetId, @Param("tagIds") List<String> tagIds);

    int deleteAssetTagLinksByAssetId(String assetId);
}
