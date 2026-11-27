package com.dasi.ppt.agent.infrastructure.persistence.mapper;

import com.dasi.ppt.agent.infrastructure.persistence.po.AssetPo;
import org.apache.ibatis.annotations.Param;

import java.util.List;

public interface AssetMapper {

    List<AssetPo> listAssets(@Param("type") String type, @Param("query") String query);

    AssetPo findAssetById(String assetId);

    String findAssetTypeById(String assetId);

    boolean existsAssetByTypeAndName(@Param("type") String type, @Param("name") String name, @Param("excludeAssetId") String excludeAssetId);

    void insertAsset(AssetPo asset);

    int updateAsset(AssetPo asset);

    void updateAssetTimestampsByTagId(@Param("tagId") String tagId, @Param("updatedAt") long updatedAt);

    boolean isUsedAsProjectTheme(String assetId);

    int deleteAsset(String assetId);
}
