package com.dasi.ppt.agent.configuration;

import com.dasi.ppt.agent.infrastructure.persistence.mapper.ProjectMapper;
import org.mybatis.spring.annotation.MapperScan;
import org.springframework.context.annotation.Configuration;

@Configuration(proxyBeanMethods = false)
@MapperScan(basePackageClasses = ProjectMapper.class)
public class PersistenceConfiguration {

}
