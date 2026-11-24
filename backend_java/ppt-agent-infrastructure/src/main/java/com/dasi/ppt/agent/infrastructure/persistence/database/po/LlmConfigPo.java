package com.dasi.ppt.agent.infrastructure.persistence.database.po;

import lombok.AllArgsConstructor;
import lombok.Builder;
import lombok.Getter;
import lombok.NoArgsConstructor;
import lombok.Setter;

@Builder
@Getter
@Setter
@NoArgsConstructor
@AllArgsConstructor
public class LlmConfigPo {

    private String id;
    private String name;
    private String protocol;
    private String provider;
    private String baseUrl;
    private String model;
    private String key;
}
