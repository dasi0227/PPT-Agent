package com.dasi.ppt.agent.domain.project.model.entity;

import com.dasi.ppt.agent.domain.project.model.enumeration.DecorationPosition;
import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Data;
import lombok.NoArgsConstructor;

@Builder
@Data
@AllArgsConstructor
@NoArgsConstructor
public final class Project {
    private String id;
    private String title;
    private String theme;
    private DecorationPosition slidePaginationPosition;
    private DecorationPosition deckTitlePosition;
    private DecorationPosition chapterTitlePosition;
    private DecorationPosition logoPosition;
    // 写入 Logo 时携带 PNG 字节，外观查询只填充 logoPresent。
    private byte[] logo;
    private boolean logoPresent;
    private long updatedAt;
}
