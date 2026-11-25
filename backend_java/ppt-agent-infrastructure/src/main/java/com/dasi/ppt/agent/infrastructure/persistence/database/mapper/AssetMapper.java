package com.dasi.ppt.agent.infrastructure.persistence.database.mapper;

import com.dasi.ppt.agent.domain.asset.model.entity.Asset;
import com.dasi.ppt.agent.infrastructure.persistence.database.po.AssetPo;
import org.apache.ibatis.annotations.Param;

import java.util.List;

public interface AssetMapper {

    List<AssetPo> listAssets(@Param("type") String type, @Param("query") String query);

    AssetPo findAssetById(String id);

    String findAssetTypeById(String assetId);

    boolean existsAssetByTypeAndName(@Param("type") String type, @Param("name") String name, @Param("excludeAssetId") String excludeAssetId);

    void insertAsset(AssetPo asset);

    int updateAsset(@Param("asset") Asset asset, @Param("now") long now);

    void updateAssetTimestampsByTagId(@Param("tagId") String tagId, @Param("updatedAt") long updatedAt);

    boolean isUsedAsProjectTheme(String id);

    int deleteAsset(String id);
}
