package com.dasi.ppt.agent.infrastructure.repository;

import com.dasi.ppt.agent.domain.asset.model.entity.Asset;
import com.dasi.ppt.agent.domain.asset.model.entity.Tag;
import com.dasi.ppt.agent.domain.asset.model.enumeration.AssetType;
import com.dasi.ppt.agent.domain.asset.repository.IAssetRepository;
import com.dasi.ppt.agent.domain.shared.exception.ErrorCode;
import com.dasi.ppt.agent.domain.shared.exception.InvalidEnumValueException;
import com.dasi.ppt.agent.infrastructure.persistence.database.mapper.AssetMapper;
import com.dasi.ppt.agent.infrastructure.persistence.database.mapper.AssetTagMapper;
import com.dasi.ppt.agent.infrastructure.persistence.database.mapper.TagMapper;
import com.dasi.ppt.agent.infrastructure.persistence.database.po.AssetPo;
import com.dasi.ppt.agent.infrastructure.persistence.database.po.TagPo;
import lombok.RequiredArgsConstructor;
import org.springframework.dao.DataRetrievalFailureException;
import org.springframework.stereotype.Repository;

import java.util.HashSet;
import java.util.List;

import static com.dasi.ppt.agent.domain.shared.exception.BusinessException.notFound;

@Repository
@RequiredArgsConstructor
public class AssetRepository implements IAssetRepository {

    private final AssetMapper assetMapper;
    private final TagMapper tagMapper;
    private final AssetTagMapper assetTagMapper;

    @Override
    public List<Asset> listAssets(AssetType type, String query) {
        return assetMapper.list(type.getValue(), query)
                .stream()
                .map(AssetRepository::toAsset)
                .toList();
    }

    @Override
    public Asset findAsset(String assetId) {
        AssetPo row = assetMapper.find(assetId);
        if (row == null) {
            throw notFound(ErrorCode.ASSET_NOT_FOUND);
        }
        return toAsset(row);
    }

    @Override
    public boolean exists(String assetId, AssetType type) {
        return assetMapper.exists(assetId, type.getValue());
    }

    @Override
    public List<Tag> listTags(AssetType type) {
        return tagMapper.list(type.getValue())
                .stream()
                .map(row -> Tag.builder()
                        .id(row.getId())
                        .type(type)
                        .name(row.getName())
                        .build())
                .toList();
    }

    @Override
    public int countTags(AssetType type, List<String> tagIds) {
        return tagMapper.countByIds(type.getValue(), tagIds);
    }

    @Override
    public boolean existTagName(AssetType type, String name) {
        return tagMapper.nameExists(type.getValue(), name);
    }

    @Override
    public void insertTag(Tag tag) {
        tagMapper.insert(TagPo.builder()
                .id(tag.getId())
                .type(tag.getType().getValue())
                .name(tag.getName())
                .build());
    }

    @Override
    public void deleteTag(String tagId) {
        assetMapper.updateTimestampsByTag(tagId, System.currentTimeMillis());
        if (tagMapper.delete(tagId) == 0) {
            throw notFound(ErrorCode.TAG_NOT_FOUND);
        }
    }

    @Override
    public boolean existName(AssetType type, String name, String excludeAssetId) {
        return assetMapper.nameExists(type.getValue(), name, excludeAssetId);
    }

    @Override
    public void insert(Asset asset) {
        assetMapper.insert(toPo(asset));
        List<String> tagIds = asset.getTagIds();
        if (tagIds != null && !tagIds.isEmpty()) {
            assetTagMapper.insert(asset.getId(), tagIds);
        }
    }

    @Override
    public void update(Asset asset) {
        String id = asset.getId();
        List<String> tagIds = asset.getTagIds();
        boolean tagsChanged = tagIds != null && !new HashSet<>(assetTagMapper.listIds(id)).equals(new HashSet<>(tagIds));
        if (assetMapper.update(asset, tagsChanged, System.currentTimeMillis()) == 0) {
            throw notFound(ErrorCode.ASSET_NOT_FOUND);
        }
        if (tagsChanged) {
            assetTagMapper.deleteExcept(id, tagIds);
            if (!tagIds.isEmpty()) {
                assetTagMapper.insert(id, tagIds);
            }
        }
    }

    @Override
    public boolean isUsed(String assetId) {
        return assetMapper.isUsed(assetId);
    }

    @Override
    public void delete(String assetId) {
        if (assetMapper.delete(assetId) == 0) {
            throw notFound(ErrorCode.ASSET_NOT_FOUND);
        }
    }

    private static Asset toAsset(AssetPo row) {
        try {
            return Asset.builder()
                    .id(row.getId())
                    .type(AssetType.fromValue(row.getType()))
                    .name(row.getName())
                    .description(row.getDescription())
                    .content(row.getContent())
                    .enabled(row.isEnabled())
                    .tagIds(row.getTagIds())
                    .updatedAt(row.getUpdatedAt())
                    .build();
        } catch (InvalidEnumValueException ex) {
            throw new DataRetrievalFailureException("assets.type 包含无效值", ex);
        }
    }

    private static AssetPo toPo(Asset asset) {
        return AssetPo.builder()
                .id(asset.getId())
                .type(asset.getType().getValue())
                .name(asset.getName())
                .description(asset.getDescription())
                .content(asset.getContent())
                .enabled(asset.getEnabled())
                .updatedAt(asset.getUpdatedAt())
                .build();
    }
}
