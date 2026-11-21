package com.dasi.ppt.agent.infrastructure.persistence.database.po;

import lombok.Getter;
import lombok.Setter;

@Getter
@Setter
public class LlmConfigPo {

    private String name;
    private String protocol;
    private String baseUrl;
    private String model;
    private String key;
}
