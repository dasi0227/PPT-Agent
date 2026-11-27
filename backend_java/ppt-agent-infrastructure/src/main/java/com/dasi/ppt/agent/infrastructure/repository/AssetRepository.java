package com.dasi.ppt.agent.infrastructure.repository;

import com.dasi.ppt.agent.domain.asset.model.entity.Asset;
import com.dasi.ppt.agent.domain.asset.model.entity.Tag;
import com.dasi.ppt.agent.domain.asset.model.enumeration.AssetType;
import com.dasi.ppt.agent.domain.asset.repository.IAssetRepository;
import com.dasi.ppt.agent.infrastructure.persistence.database.mapper.AssetMapper;
import com.dasi.ppt.agent.infrastructure.persistence.database.mapper.AssetTagMapper;
import com.dasi.ppt.agent.infrastructure.persistence.database.mapper.TagMapper;
import com.dasi.ppt.agent.infrastructure.persistence.database.po.AssetPo;
import com.dasi.ppt.agent.infrastructure.persistence.database.po.TagPo;
import lombok.RequiredArgsConstructor;
import org.springframework.stereotype.Repository;

import java.util.List;

@Repository
@RequiredArgsConstructor
public class AssetRepository implements IAssetRepository {

    private final AssetMapper assetMapper;
    private final TagMapper tagMapper;
    private final AssetTagMapper assetTagMapper;

    @Override
    public List<Asset> listAssets(AssetType type, String query) {
        return assetMapper.listAssets(type.getValue(), query)
                .stream()
                .map(AssetRepository::toEntity)
                .toList();
    }

    @Override
    public Asset findAssetById(String assetId) {
        return toEntity(assetMapper.findAssetById(assetId));
    }

    @Override
    public AssetType findAssetTypeById(String assetId) {
        String value = assetMapper.findAssetTypeById(assetId);
        return value == null ? null : AssetType.fromValue(value);
    }

    @Override
    public List<Tag> listTags(AssetType type) {
        return tagMapper.listTags(type.getValue())
                .stream()
                .map(row -> Tag.builder()
                        .id(row.getId())
                        .type(type)
                        .name(row.getName())
                        .build())
                .toList();
    }

    @Override
    public int countTagsByTypeAndIds(AssetType type, List<String> tagIds) {
        return tagMapper.countTagsByTypeAndIds(type.getValue(), tagIds);
    }

    @Override
    public boolean existsTagByTypeAndName(AssetType type, String name) {
        return tagMapper.existsTagByTypeAndName(type.getValue(), name);
    }

    @Override
    public void insertTag(Tag tag) {
        tagMapper.insertTag(TagPo.builder()
                .id(tag.getId())
                .type(tag.getType().getValue())
                .name(tag.getName())
                .build());
    }

    @Override
    public boolean existsConflictingTagName(String tagId, String name) {
        return tagMapper.existsConflictingTagName(tagId, name);
    }

    @Override
    public int updateTagName(String tagId, String name) {
        return tagMapper.updateTagName(tagId, name);
    }

    @Override
    public int deleteTag(String tagId, long updatedAt) {
        assetMapper.updateAssetTimestampsByTagId(tagId, updatedAt);
        return tagMapper.deleteTag(tagId);
    }

    @Override
    public boolean existsAssetByTypeAndName(AssetType type, String name, String excludeAssetId) {
        return assetMapper.existsAssetByTypeAndName(type.getValue(), name, excludeAssetId);
    }

    @Override
    public void insertAsset(Asset asset) {
        assetMapper.insertAsset(toPo(asset));
        List<String> tagIds = asset.getTagIds();
        if (tagIds != null && !tagIds.isEmpty()) {
            assetTagMapper.insertAssetTagLinks(asset.getId(), tagIds);
        }
    }

    @Override
    public void updateAsset(Asset asset) {
        assetMapper.updateAsset(toPo(asset));
        List<String> tagIds = asset.getTagIds();
        if (tagIds != null) {
            String assetId = asset.getId();
            assetTagMapper.deleteAssetTagLinksByAssetId(assetId);
            if (!tagIds.isEmpty()) {
                assetTagMapper.insertAssetTagLinks(assetId, tagIds);
            }
        }
    }

    @Override
    public boolean isUsedAsProjectTheme(String assetId) {
        return assetMapper.isUsedAsProjectTheme(assetId);
    }

    @Override
    public int deleteAsset(String assetId) {
        return assetMapper.deleteAsset(assetId);
    }

    private static Asset toEntity(AssetPo po) {
        if (po == null) return null;
        return Asset.builder()
                .id(po.getId())
                .type(AssetType.fromValue(po.getType()))
                .name(po.getName())
                .description(po.getDescription())
                .content(po.getContent())
                .enabled(po.getEnabled())
                .tagIds(po.getTagIds())
                .updatedAt(po.getUpdatedAt())
                .build();
    }

    private static AssetPo toPo(Asset asset) {
        return AssetPo.builder()
                .id(asset.getId())
                .type(asset.getType() == null ? null : asset.getType().getValue())
                .name(asset.getName())
                .description(asset.getDescription())
                .content(asset.getContent())
                .enabled(asset.getEnabled())
                .updatedAt(asset.getUpdatedAt())
                .build();
    }
}
