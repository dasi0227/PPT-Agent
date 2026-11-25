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

    public List<Tag> listTags(AssetType type) {
        return assetRepository.listTags(type);
    }

    @Transactional
    public String createTag(AssetType type, String name) {
        if (assetRepository.existTagName(type, name)) {
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
    public void deleteTag(String tagId) {
        assetRepository.deleteTag(tagId);
    }

    public Asset findAsset(String id) {
        return assetRepository.findAsset(id);
    }

    @Transactional
    public String createAsset(Asset asset) {
        if (assetRepository.existName(asset.getType(), asset.getName(), null)) {
            throw invalid("同类型资产名称已存在");
        }
        List<String> tagIds = asset.getTagIds();
        if (tagIds != null && !tagIds.isEmpty() && assetRepository.countTags(asset.getType(), tagIds) != tagIds.size()) {
            throw invalid("标签需存在、同类且不重复");
        }
        asset.setId(IdUtil.genAssetId());
        asset.setEnabled(true);
        asset.setUpdatedAt(System.currentTimeMillis());
        assetRepository.insert(asset);
        return asset.getId();
    }

    @Transactional
    public void updateAsset(Asset asset) {
        if (!assetRepository.exists(asset.getId(), asset.getType())) {
            throw notFound(ErrorCode.ASSET_NOT_FOUND);
        }
        if (asset.getName() != null && assetRepository.existName(asset.getType(), asset.getName(), asset.getId())) {
            throw invalid("同类型资产名称已存在");
        }
        if (asset.getType() == AssetType.THEME && Boolean.FALSE.equals(asset.getEnabled()) && assetRepository.isUsed(asset.getId())) {
            throw conflict(ErrorCode.ASSET_IN_USE, "项目正在使用该主题，无法停用");
        }
        List<String> tagIds = asset.getTagIds();
        if (tagIds != null && !tagIds.isEmpty() && assetRepository.countTags(asset.getType(), tagIds) != tagIds.size()) {
            throw invalid("标签需存在、同类且不重复");
        }
        assetRepository.update(asset);
    }

    @Transactional
    public void delete(String id) {
        if (assetRepository.isUsed(id)) {
            throw conflict(ErrorCode.ASSET_IN_USE);
        }
        assetRepository.delete(id);
    }
}
