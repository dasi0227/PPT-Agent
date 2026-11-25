package com.dasi.ppt.agent.domain.asset.service;

import com.dasi.ppt.agent.domain.asset.model.entity.Asset;
import com.dasi.ppt.agent.domain.asset.model.entity.Tag;
import com.dasi.ppt.agent.domain.asset.model.enumeration.AssetType;
import com.dasi.ppt.agent.domain.asset.repository.IAssetRepository;
import com.dasi.ppt.agent.domain.shared.exception.ErrorCode;
import com.dasi.ppt.agent.domain.shared.util.IdUtil;
import lombok.RequiredArgsConstructor;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;

import java.util.List;

import static com.dasi.ppt.agent.domain.shared.exception.BusinessException.*;

@Service
@RequiredArgsConstructor
public class AssetService {

    private final IAssetRepository assetRepository;

    public List<Asset> listAssets(AssetType type, String query) {
        return assetRepository.listAssets(type, query == null ? "" : query);
    }

    public Asset getAssetDetail(String assetId) {
        Asset asset = assetRepository.findAssetById(assetId);
        if (asset == null) {
            throw notFound(ErrorCode.ASSET_NOT_FOUND);
        }
        return asset;
    }

    @Transactional
    public String createAsset(Asset asset) {
        if (assetRepository.existsAssetByTypeAndName(asset.getType(), asset.getName(), null)) {
            throw invalid("同类型资产名称已存在");
        }
        List<String> tagIds = asset.getTagIds();
        if (tagIds != null && !tagIds.isEmpty() && assetRepository.countTagsByTypeAndIds(asset.getType(), tagIds) != tagIds.size()) {
            throw invalid("标签需存在、同类且不重复");
        }
        asset.setId(IdUtil.genAssetId());
        asset.setEnabled(true);
        asset.setUpdatedAt(System.currentTimeMillis());
        assetRepository.insertAsset(asset);
        return asset.getId();
    }

    @Transactional
    public void updateAsset(Asset asset) {
        AssetType type = assetRepository.findAssetTypeById(asset.getId());
        if (type == null) {
            throw notFound(ErrorCode.ASSET_NOT_FOUND);
        }
        if (asset.getName() != null && assetRepository.existsAssetByTypeAndName(type, asset.getName(), asset.getId())) {
            throw invalid("同类型资产名称已存在");
        }
        if (type == AssetType.THEME && Boolean.FALSE.equals(asset.getEnabled()) && assetRepository.isUsedAsProjectTheme(asset.getId())) {
            throw conflict(ErrorCode.ASSET_IN_USE, "项目正在使用该主题，无法停用");
        }
        List<String> tagIds = asset.getTagIds();
        if (tagIds != null && !tagIds.isEmpty() && assetRepository.countTagsByTypeAndIds(type, tagIds) != tagIds.size()) {
            throw invalid("标签需存在、同类且不重复");
        }
        asset.setUpdatedAt(System.currentTimeMillis());
        assetRepository.updateAsset(asset);
    }

    @Transactional
    public void deleteAsset(String assetId) {
        if (assetRepository.isUsedAsProjectTheme(assetId)) {
            throw conflict(ErrorCode.ASSET_IN_USE);
        }
        if (assetRepository.deleteAsset(assetId) == 0) {
            throw notFound(ErrorCode.ASSET_NOT_FOUND);
        }
    }

    public List<Tag> listTags(AssetType type) {
        return assetRepository.listTags(type);
    }

    @Transactional
    public String createTag(AssetType type, String name) {
        if (assetRepository.existsTagByTypeAndName(type, name)) {
            throw invalid("同类型标签名称已存在");
        }
        Tag tag = Tag.builder()
                .id(IdUtil.genTagId())
                .type(type)
                .name(name)
                .build();
        assetRepository.insertTag(tag);
        return tag.getId();
    }

    @Transactional
    public void updateTag(String tagId, String name) {
        if (assetRepository.existsConflictingTagName(tagId, name)) {
            throw invalid("同类型标签名称已存在");
        }
        if (assetRepository.updateTagName(tagId, name) == 0) {
            throw notFound(ErrorCode.TAG_NOT_FOUND);
        }
    }

    @Transactional
    public void deleteTag(String tagId) {
        if (assetRepository.deleteTag(tagId) == 0) {
            throw notFound(ErrorCode.TAG_NOT_FOUND);
        }
    }
}
