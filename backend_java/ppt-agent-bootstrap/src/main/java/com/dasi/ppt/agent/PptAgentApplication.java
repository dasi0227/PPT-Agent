package com.dasi.ppt.agent;

import org.springframework.boot.SpringApplication;
import org.springframework.boot.autoconfigure.SpringBootApplication;
import org.springframework.boot.context.properties.ConfigurationPropertiesScan;

@SpringBootApplication
@ConfigurationPropertiesScan
public class PptAgentApplication {

    public static void main(String[] args) {
        SpringApplication.run(PptAgentApplication.class, args);
    }

}
