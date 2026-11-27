package com.dasi.ppt.agent.infrastructure.persistence.po;

import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Getter;
import lombok.NoArgsConstructor;
import lombok.Setter;

import java.util.List;

@Builder
@Getter
@Setter
@AllArgsConstructor
@NoArgsConstructor
public class AssetPo {

    private String id;
    private String type;
    private String name;
    private String description;
    private String content;
    private Boolean enabled;
    private long updatedAt;
    private List<String> tagIds;
}
