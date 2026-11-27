package com.dasi.ppt.agent.domain.shared.service;

import com.dasi.ppt.agent.domain.project.model.enumeration.DeckFileType;
import com.networknt.schema.Error;
import com.networknt.schema.Schema;
import com.networknt.schema.SchemaRegistry;
import com.networknt.schema.SchemaRegistryConfig;
import com.networknt.schema.SpecificationVersion;
import lombok.RequiredArgsConstructor;
import org.springframework.core.io.ClassPathResource;
import org.springframework.stereotype.Service;
import tools.jackson.core.JacksonException;
import tools.jackson.databind.JsonNode;
import tools.jackson.databind.ObjectMapper;

import java.io.IOException;
import java.io.InputStream;
import java.io.UncheckedIOException;
import java.util.EnumMap;
import java.util.List;
import java.util.Map;

import static com.dasi.ppt.agent.domain.shared.exception.BusinessException.illegal;
import static com.dasi.ppt.agent.domain.shared.exception.BusinessException.invalid;

@Service
@RequiredArgsConstructor
public class SchemaService {

    private final ObjectMapper objectMapper;
    private final Map<DeckFileType, Schema> schemas = loadSchemas();

    private static Map<DeckFileType, Schema> loadSchemas() {
        Map<DeckFileType, Schema> schemas = new EnumMap<>(DeckFileType.class);
        SchemaRegistryConfig config = SchemaRegistryConfig.builder().errorMessageKeyword("errorMessage").build();
        SchemaRegistry registry = SchemaRegistry.withDefaultDialect(SpecificationVersion.DRAFT_2020_12, builder -> builder.schemaRegistryConfig(config));
        for (DeckFileType fileType : List.of(DeckFileType.META, DeckFileType.CONSTRAINT, DeckFileType.OUTLINE, DeckFileType.SPEC)) {
            String path = "schemas/" + fileType.getValue() + ".schema.json";
            try (InputStream input = new ClassPathResource(path).getInputStream()) {
                schemas.put(fileType, registry.getSchema(input));
            } catch (IOException ex) {
                throw new UncheckedIOException(ex);
            }
        }
        return schemas;
    }

    public void validate(DeckFileType fileType, byte[] content) {
        List<Error> errors = schemas.get(fileType).validate(objectMapper.readTree(content));
        if (!errors.isEmpty()) {
            throw invalid(errors.get(0).toString());
        }
    }

    public <T> T decode(DeckFileType fileType, byte[] bytes, Class<T> type) {
        if (bytes == null) return null;
        try {
            JsonNode json = objectMapper.readTree(bytes);
            if (!schemas.get(fileType).validate(json).isEmpty()) {
                throw illegal("文稿文件格式无效：" + fileType.getValue());
            }
            return objectMapper.treeToValue(json, type);
        } catch (JacksonException ex) {
            throw illegal("文稿文件格式无效：" + fileType.getValue());
        }
    }
}
