package com.dasi.ppt.agent.configuration;

import com.dasi.ppt.agent.domain.project.repository.IProjectRepository;
import com.dasi.ppt.agent.infrastructure.persistence.database.mapper.ProjectMapper;
import org.mybatis.spring.annotation.MapperScan;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.springframework.boot.ApplicationRunner;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;

@Configuration(proxyBeanMethods = false)
@MapperScan(basePackageClasses = ProjectMapper.class)
public class PersistenceConfiguration {

    private static final Logger log = LoggerFactory.getLogger(PersistenceConfiguration.class);

    @Bean
    ApplicationRunner checkPublicDatabase(IProjectRepository projects) {
        return args -> {
            if (!projects.isDatabaseAvailable()) {
                log.warn("SQLite 数据库或表结构不可用，请检查配置的数据库路径和外部建表脚本");
            }
        };
    }
}
