package com.dasi.ppt.agent.interfaces.controller.asset.vo;

import com.dasi.ppt.agent.domain.asset.model.entity.Tag;
import lombok.Builder;
import lombok.Getter;

import java.util.List;

@Builder
@Getter
public final class TagListVo {
    private List<TagInfoVo> items;

    @Builder
    @Getter
    public static final class TagInfoVo {
        private String id;
        private String name;

        public static TagInfoVo toVo(Tag tag) {
            return TagInfoVo.builder()
                    .id(tag.getId())
                    .name(tag.getName())
                    .build();
        }
    }
}
