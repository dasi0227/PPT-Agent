package com.dasi.ppt.agent.domain.asset.model.entity;

import com.dasi.ppt.agent.domain.asset.model.enumeration.AssetType;
import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Data;
import lombok.NoArgsConstructor;

import java.nio.charset.StandardCharsets;
import java.util.HashSet;
import java.util.List;
import java.util.Set;

import static com.dasi.ppt.agent.domain.shared.exception.BusinessException.invalid;

@Builder
@Data
@AllArgsConstructor
@NoArgsConstructor
public final class Asset {
    private String id;
    private AssetType type;
    private String name;
    private String description;
    private String content;
    private Boolean enabled;
    private List<String> tagIds;
    private long updatedAt;

    public void normalize() {
        if (tagIds != null) {
            tagIds = tagIds.stream().sorted().toList();
        }
        if (name != null) {
            name = name.strip();
        }
        if (description != null) {
            description = description.strip();
        }
    }

    public void validate() {
        if (content != null) {
            if (!StandardCharsets.UTF_8.newEncoder().canEncode(content)) {
                throw invalid("正文需为 UTF-8 原文");
            }
            if (type == AssetType.THEME || type == AssetType.SNIPPET) {
                int limit = type == AssetType.THEME ? 64 : 16;
                if (content.getBytes(StandardCharsets.UTF_8).length > limit * 1024) {
                    throw invalid("正文超过 " + limit + " KiB");
                }
            }
        }
        if (tagIds != null) {
            Set<String> seen = new HashSet<>();
            for (String tagId : tagIds) {
                if (tagId == null || !seen.add(tagId)) {
                    throw invalid("标签需存在、同类且不重复");
                }
            }
        }
    }
}
