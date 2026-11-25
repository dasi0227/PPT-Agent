package com.dasi.ppt.agent.domain.asset.repository;

import com.dasi.ppt.agent.domain.asset.model.entity.Asset;
import com.dasi.ppt.agent.domain.asset.model.entity.Tag;
import com.dasi.ppt.agent.domain.asset.model.enumeration.AssetType;

import java.util.List;

public interface IAssetRepository {

    List<Asset> listAssets(AssetType type, String query);

    Asset findAsset(String assetId);

    boolean exists(String assetId, AssetType type);

    List<Tag> listTags(AssetType type);

    int countTags(AssetType type, List<String> tagIds);

    boolean existTagName(AssetType type, String name);

    void insertTag(Tag tag);

    void deleteTag(String tagId);

    boolean existName(AssetType type, String name, String excludeAssetId);

    void insert(Asset asset);

    void update(Asset asset);

    boolean isUsed(String assetId);

    void delete(String assetId);
}
