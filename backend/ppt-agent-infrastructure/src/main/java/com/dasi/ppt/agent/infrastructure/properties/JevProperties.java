package com.dasi.ppt.agent.infrastructure.properties;

import lombok.Getter;
import lombok.Setter;
import org.springframework.boot.context.properties.ConfigurationProperties;

@Getter
@Setter
@ConfigurationProperties("ppt.jev")
public class JevProperties {

    private String model;
    private String key;
}
