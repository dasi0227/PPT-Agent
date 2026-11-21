package com.dasi.ppt.agent.infrastructure.persistence.database.po;

import lombok.Getter;
import lombok.Setter;

@Getter
@Setter
public class AssetPo {

    private String id;
    private String type;
    private String name;
    private String description;
    private String content;
    private boolean disabled;
    private long updatedAt;
}
