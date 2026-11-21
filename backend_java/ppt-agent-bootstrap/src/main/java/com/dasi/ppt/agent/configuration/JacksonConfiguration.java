package com.dasi.ppt.agent.configuration;

import org.springframework.boot.jackson.autoconfigure.JsonMapperBuilderCustomizer;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import tools.jackson.databind.PropertyNamingStrategies;

@Configuration(proxyBeanMethods = false)
public class JacksonConfiguration {

    @Bean
    JsonMapperBuilderCustomizer apiPropertyNames() {
        return builder -> builder.propertyNamingStrategy(PropertyNamingStrategies.SNAKE_CASE);
    }
}
