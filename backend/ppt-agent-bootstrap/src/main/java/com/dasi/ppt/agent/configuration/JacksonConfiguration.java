package com.dasi.ppt.agent.configuration;

import com.dasi.ppt.agent.domain.agent.service.tool.result.AskUserResult;
import org.springframework.boot.jackson.autoconfigure.JsonMapperBuilderCustomizer;
import org.springframework.context.annotation.Bean;
import org.springframework.context.annotation.Configuration;
import tools.jackson.core.JsonGenerator;
import tools.jackson.databind.DeserializationFeature;
import tools.jackson.databind.MapperFeature;
import tools.jackson.databind.PropertyNamingStrategies;
import tools.jackson.databind.SerializationContext;
import tools.jackson.databind.ValueSerializer;
import tools.jackson.databind.cfg.CoercionAction;
import tools.jackson.databind.cfg.CoercionInputShape;
import tools.jackson.databind.cfg.EnumFeature;
import tools.jackson.databind.module.SimpleModule;
import tools.jackson.databind.type.LogicalType;

import java.util.Map;

@Configuration(proxyBeanMethods = false)
public class JacksonConfiguration {

    @Bean
    JsonMapperBuilderCustomizer apiJson() {
        SimpleModule toolResults = new SimpleModule();
        toolResults.addSerializer(AskUserResult.Answer.class, new ValueSerializer<AskUserResult.Answer>() {
            @Override
            public void serialize(AskUserResult.Answer answer, JsonGenerator output, SerializationContext context) {
                output.writePOJO(answer.getText() == null ? Map.of("state", "skipped") : answer.getText());
            }
        });
        return builder -> builder.propertyNamingStrategy(PropertyNamingStrategies.SNAKE_CASE)
                .addModule(toolResults)
                .enable(DeserializationFeature.FAIL_ON_UNKNOWN_PROPERTIES)
                .disable(MapperFeature.ALLOW_COERCION_OF_SCALARS)
                .enable(EnumFeature.FAIL_ON_NUMBERS_FOR_ENUMS)
                .withCoercionConfig(LogicalType.Textual, rules -> rules
                        .setCoercion(CoercionInputShape.Integer, CoercionAction.Fail)
                        .setCoercion(CoercionInputShape.Float, CoercionAction.Fail)
                        .setCoercion(CoercionInputShape.Boolean, CoercionAction.Fail));
    }
}
