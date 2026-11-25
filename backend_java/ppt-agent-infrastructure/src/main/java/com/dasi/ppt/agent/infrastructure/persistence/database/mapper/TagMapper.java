package com.dasi.ppt.agent.infrastructure.persistence.database.mapper;

import com.dasi.ppt.agent.infrastructure.persistence.database.po.TagPo;
import org.apache.ibatis.annotations.Param;

import java.util.List;

public interface TagMapper {

    List<TagPo> list(String type);

    int countByIds(@Param("type") String type, @Param("tagIds") List<String> tagIds);

    boolean nameExists(@Param("type") String type, @Param("name") String name);

    void insert(TagPo tag);

    int delete(String tagId);
}
