package com.dasi.ppt.agent.domain.project.service;

import com.dasi.ppt.agent.domain.project.model.enumeration.ChangePart;
import com.networknt.schema.Error;
import com.networknt.schema.Schema;
import com.networknt.schema.SchemaRegistry;
import com.networknt.schema.SpecificationVersion;
import org.springframework.core.io.ClassPathResource;
import org.springframework.stereotype.Component;
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

@Component
public class DeckDocuments {

    private final ObjectMapper objectMapper;
    private final Map<ChangePart, Schema> schemas = new EnumMap<>(ChangePart.class);

    // 启动时加载 meta、constraint、outline、spec 这四份 JSON Schema
    public DeckDocuments(ObjectMapper objectMapper) {
        this.objectMapper = objectMapper;
        SchemaRegistry registry = SchemaRegistry.withDefaultDialect(SpecificationVersion.DRAFT_2020_12);
        for (ChangePart part : List.of(ChangePart.META, ChangePart.CONSTRAINT, ChangePart.OUTLINE, ChangePart.SPEC)) {
            String path = "schemas/" + part.getValue() + ".schema.json";
            try (InputStream input = new ClassPathResource(path).getInputStream()) {
                schemas.put(part, registry.getSchema(input));
            } catch (IOException ex) {
                throw new UncheckedIOException(ex);
            }
        }
    }

    public byte[] encode(ChangePart part, Object document) {
        JsonNode json = objectMapper.valueToTree(document);
        List<Error> errors = schemas.get(part).validate(json);
        if (!errors.isEmpty()) {
            throw invalid(errors.get(0).toString());
        }
        return objectMapper.writeValueAsBytes(json);
    }

    public <T> T decode(ChangePart part, byte[] bytes, Class<T> type) {
        if (bytes == null) return null;
        try {
            JsonNode json = objectMapper.readTree(bytes);
            if (!schemas.get(part).validate(json).isEmpty()) {
                throw illegal("文稿文件格式无效：" + part.getValue());
            }
            return objectMapper.treeToValue(json, type);
        } catch (JacksonException ex) {
            throw illegal("文稿文件格式无效：" + part.getValue());
        }
    }
}
