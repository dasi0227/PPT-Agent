package com.dasi.ppt.agent.domain.asset.service;

import com.dasi.ppt.agent.domain.asset.model.entity.Asset;
import com.dasi.ppt.agent.domain.asset.model.entity.Tag;
import com.dasi.ppt.agent.domain.asset.model.enumeration.AssetType;
import com.dasi.ppt.agent.domain.asset.repository.IAssetRepository;
import com.dasi.ppt.agent.domain.shared.exception.ErrorCode;
import com.dasi.ppt.agent.domain.shared.util.HashUtil;
import com.dasi.ppt.agent.domain.shared.util.IdUtil;
import lombok.RequiredArgsConstructor;
import org.springframework.stereotype.Service;
import org.springframework.transaction.annotation.Transactional;

import java.util.HashSet;
import java.util.List;
import java.util.Set;

import static com.dasi.ppt.agent.domain.shared.exception.BusinessException.conflict;
import static com.dasi.ppt.agent.domain.shared.exception.BusinessException.invalid;

@Service
@RequiredArgsConstructor
public class AssetService {

    private final IAssetRepository assetRepository;

    public List<Asset> listAssets(AssetType type, String query) {
        return assetRepository.listAssets(type, query == null ? "" : query.strip());
    }

    public List<Tag> listTags(AssetType type) {
        return assetRepository.listTags(type);
    }

    @Transactional
    public Tag createTag(AssetType type, String name) {
        String displayName = name.strip();
        if (assetRepository.existTagName(type, displayName)) {
            throw invalid("同类型标签名称已存在");
        }
        Tag tag = Tag.builder()
                .id(IdUtil.genTagId())
                .type(type)
                .name(displayName)
                .build();
        assetRepository.insertTag(tag);
        return tag;
    }

    @Transactional
    public void deleteTag(String tagId) {
        assetRepository.deleteTag(tagId);
    }

    public Asset findAsset(String id) {
        return assetRepository.findAsset(id);
    }

    @Transactional
    public Asset createAsset(Asset asset) {
        asset.validate();
        asset.normalize();
        validateNameAndTags(asset, null);

        asset.setId(IdUtil.genAssetId());
        asset.setEnabled(true);
        if (asset.getTagIds() == null) {
            asset.setTagIds(List.of());
        }
        asset.setUpdatedAt(System.currentTimeMillis());
        assetRepository.insert(asset);
        return asset;
    }

    @Transactional
    public Asset updateAsset(Asset asset, String baseHash) {
        Asset current = findAsset(asset.getId());
        if (asset.getContent() != null && !HashUtil.calAssetHash(current).equals(baseHash)) {
            throw conflict(ErrorCode.CONTENT_CONFLICT, "资产正文已更新，请重新读取后保存");
        }

        asset.setType(current.getType());
        asset.validate();
        asset.normalize();
        validateNameAndTags(asset, current.getId());

        String name = asset.getName() == null ? current.getName() : asset.getName();
        String description = asset.getDescription() == null ? current.getDescription() : asset.getDescription();
        String content = asset.getContent() == null ? current.getContent() : asset.getContent();
        boolean enabled = asset.getEnabled() == null ? current.getEnabled() : asset.getEnabled();
        List<String> tags = asset.getTagIds() == null ? current.getTagIds() : asset.getTagIds();

        boolean tagsChanged = !tags.equals(current.getTagIds());
        if (name.equals(current.getName()) && description.equals(current.getDescription())
                && content.equals(current.getContent()) && enabled == current.getEnabled() && !tagsChanged) {
            return current;
        }
        if (current.getType() == AssetType.THEME && current.getEnabled() && !enabled && assetRepository.isUsed(current.getId())) {
            throw conflict(ErrorCode.ASSET_IN_USE, "项目正在使用该主题，无法停用");
        }
        Asset updated = Asset.builder()
                .id(current.getId())
                .type(current.getType())
                .name(name)
                .description(description)
                .content(content)
                .enabled(enabled)
                .tagIds(tags)
                .updatedAt(Math.max(System.currentTimeMillis(), current.getUpdatedAt() + 1))
                .build();
        assetRepository.update(updated, tagsChanged);
        return updated;
    }

    @Transactional
    public void delete(String id) {
        if (assetRepository.isUsed(id)) {
            throw conflict(ErrorCode.ASSET_IN_USE);
        }
        assetRepository.delete(id);
    }

    private void validateNameAndTags(Asset asset, String excludeAssetId) {
        if (asset.getName() != null && assetRepository.existName(asset.getType(), asset.getName(), excludeAssetId)) {
            throw invalid("同类型资产名称已存在");
        }
        List<String> tagIds = asset.getTagIds();
        if (tagIds == null || tagIds.isEmpty()) {
            return;
        }
        Set<String> validTags = new HashSet<>();
        assetRepository.listTags(asset.getType()).forEach(tag -> validTags.add(tag.getId()));
        if (!validTags.containsAll(tagIds)) {
            throw invalid("标签需存在、同类且不重复");
        }
    }
}
