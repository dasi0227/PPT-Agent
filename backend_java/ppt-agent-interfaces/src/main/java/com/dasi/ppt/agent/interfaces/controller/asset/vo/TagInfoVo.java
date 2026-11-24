package com.dasi.ppt.agent.interfaces.controller.asset.vo;

import com.dasi.ppt.agent.domain.asset.model.entity.Tag;
import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Getter;
import lombok.NoArgsConstructor;

@Builder
@Getter
@AllArgsConstructor
@NoArgsConstructor
public final class TagInfoVo {
    private String id;
    private String name;

    public static TagInfoVo toVo(Tag tag) {
        return TagInfoVo.builder()
                .id(tag.getId())
                .name(tag.getName())
                .build();
    }
}
