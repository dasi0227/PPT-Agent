package com.dasi.ppt.agent.domain.asset.repository;

import com.dasi.ppt.agent.domain.asset.model.entity.Asset;
import com.dasi.ppt.agent.domain.asset.model.entity.Tag;
import com.dasi.ppt.agent.domain.asset.model.enumeration.AssetType;

import java.util.List;

public interface IAssetRepository {

    List<Asset> listAssets(AssetType type, String query);

    Asset findAssetById(String assetId);

    AssetType findAssetTypeById(String assetId);

    List<Tag> listTags(AssetType type);

    int countTagsByTypeAndIds(AssetType type, List<String> tagIds);

    boolean existsTagByTypeAndName(AssetType type, String name);

    void insertTag(Tag tag);

    boolean existsConflictingTagName(String tagId, String name);

    int updateTagName(String tagId, String name);

    int deleteTag(String tagId);

    boolean existsAssetByTypeAndName(AssetType type, String name, String excludeAssetId);

    void insertAsset(Asset asset);

    void updateAsset(Asset asset);

    boolean isUsedAsProjectTheme(String assetId);

    int deleteAsset(String assetId);
}
