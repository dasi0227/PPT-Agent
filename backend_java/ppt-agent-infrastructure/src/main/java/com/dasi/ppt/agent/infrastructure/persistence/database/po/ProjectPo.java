package com.dasi.ppt.agent.infrastructure.persistence.database.po;

import lombok.Getter;
import lombok.Setter;

@Getter
@Setter
public class ProjectPo {

    private String id;
    private String title;
    private String theme;
    private byte[] logo;
    private String slidePaginationPosition;
    private String deckTitlePosition;
    private String sectionTitlePosition;
    private String projectLogoPosition;
    private long updatedAt;
}
