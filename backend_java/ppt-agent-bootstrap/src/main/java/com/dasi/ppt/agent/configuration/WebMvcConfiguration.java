package com.dasi.ppt.agent.configuration;

import lombok.RequiredArgsConstructor;
import org.springframework.context.annotation.Configuration;
import org.springframework.core.task.AsyncTaskExecutor;
import org.springframework.web.servlet.config.annotation.AsyncSupportConfigurer;
import org.springframework.web.servlet.config.annotation.WebMvcConfigurer;

@Configuration(proxyBeanMethods = false)
@RequiredArgsConstructor
public class WebMvcConfiguration implements WebMvcConfigurer {

    private final AsyncTaskExecutor applicationTaskExecutor;

    @Override
    public void configureAsyncSupport(AsyncSupportConfigurer configurer) {
        configurer.setTaskExecutor(applicationTaskExecutor);
    }
}
