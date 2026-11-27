package com.dasi.ppt.agent.configuration;

import org.springframework.ai.anthropic.AnthropicChatModel;
import org.springframework.ai.openai.responses.OpenAiResponsesChatModel;
import org.springframework.beans.factory.config.ConfigurableBeanFactory;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import org.springframework.context.annotation.Scope;

import java.net.http.HttpClient;

@Configuration(proxyBeanMethods = false)
public class LlmClientConfiguration {

    @Bean
    HttpClient jevHttpClient() {
        return HttpClient.newHttpClient();
    }

    @Bean
    @Scope(ConfigurableBeanFactory.SCOPE_PROTOTYPE)
    OpenAiResponsesChatModel.Builder responsesModelBuilder() {
        return OpenAiResponsesChatModel.builder();
    }

    @Bean
    @Scope(ConfigurableBeanFactory.SCOPE_PROTOTYPE)
    AnthropicChatModel.Builder anthropicModelBuilder() {
        return AnthropicChatModel.builder();
    }
}
