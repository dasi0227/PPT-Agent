package com.dasi.ppt.agent.infrastructure.persistence.mapper;

import com.dasi.ppt.agent.infrastructure.persistence.po.TagPo;
import org.apache.ibatis.annotations.Param;

import java.util.List;

public interface TagMapper {

    List<TagPo> listTags(String type);

    int countTagsByTypeAndIds(@Param("type") String type, @Param("tagIds") List<String> tagIds);

    boolean existsTagByTypeAndName(@Param("type") String type, @Param("name") String name);

    void insertTag(TagPo tag);

    boolean existsConflictingTagName(@Param("tagId") String tagId, @Param("name") String name);

    int updateTagName(@Param("tagId") String tagId, @Param("name") String name);

    int deleteTag(String tagId);
}
