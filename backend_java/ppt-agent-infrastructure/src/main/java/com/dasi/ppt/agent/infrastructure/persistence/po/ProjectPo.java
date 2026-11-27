package com.dasi.ppt.agent.infrastructure.persistence.po;

import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Getter;
import lombok.NoArgsConstructor;

@Builder
@Getter
@AllArgsConstructor
@NoArgsConstructor
public class ProjectPo {

    private String id;
    private String title;
    private String theme;
    private byte[] logo;
    private boolean logoPresent;
    private String slidePaginationPosition;
    private String deckTitlePosition;
    private String chapterTitlePosition;
    private String projectLogoPosition;
    private long updatedAt;
}
