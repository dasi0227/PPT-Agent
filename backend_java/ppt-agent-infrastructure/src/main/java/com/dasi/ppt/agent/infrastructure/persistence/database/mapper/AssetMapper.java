package com.dasi.ppt.agent.infrastructure.persistence.database.mapper;

import com.dasi.ppt.agent.infrastructure.persistence.database.po.AssetPo;
import org.apache.ibatis.annotations.Param;

import java.util.List;

public interface AssetMapper {

    List<AssetPo> list(@Param("type") String type, @Param("query") String query);

    AssetPo find(String id);

    List<String> listNames(@Param("type") String type, @Param("excludeAssetId") String excludeAssetId);

    void insert(AssetPo asset);

    int update(AssetPo asset);

    void updateTimestampsByTag(@Param("tagId") String tagId, @Param("updatedAt") long updatedAt);

    boolean isUsed(String id);

    int delete(String id);
}
