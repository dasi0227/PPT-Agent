package com.dasi.ppt.agent.infrastructure.persistence.database.po;

import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Getter;
import lombok.NoArgsConstructor;
import lombok.Setter;

@Builder
@Getter
@Setter
@AllArgsConstructor
@NoArgsConstructor
public class AssetTagPo {

    private String assetId;
    private String tagId;
}
