package com.dasi.ppt.agent.infrastructure.persistence.database.mapper;

import com.dasi.ppt.agent.infrastructure.persistence.database.po.TagPo;

import java.util.List;

public interface TagMapper {

    List<TagPo> list(String type);

    List<String> listNames(String type);

    void insert(TagPo tag);

    int delete(String tagId);
}
