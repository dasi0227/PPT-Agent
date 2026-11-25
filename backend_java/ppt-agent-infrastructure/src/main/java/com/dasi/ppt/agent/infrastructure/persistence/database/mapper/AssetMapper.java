package com.dasi.ppt.agent.infrastructure.persistence.database.mapper;

import com.dasi.ppt.agent.domain.asset.model.entity.Asset;
import com.dasi.ppt.agent.infrastructure.persistence.database.po.AssetPo;
import org.apache.ibatis.annotations.Param;

import java.util.List;

public interface AssetMapper {

    List<AssetPo> list(@Param("type") String type, @Param("query") String query);

    AssetPo find(String id);

    boolean exists(@Param("id") String id, @Param("type") String type);

    boolean nameExists(@Param("type") String type, @Param("name") String name, @Param("excludeAssetId") String excludeAssetId);

    void insert(AssetPo asset);

    int update(@Param("asset") Asset asset, @Param("tagsChanged") boolean tagsChanged, @Param("now") long now);

    void updateTimestampsByTag(@Param("tagId") String tagId, @Param("updatedAt") long updatedAt);

    boolean isUsed(String id);

    int delete(String id);
}
