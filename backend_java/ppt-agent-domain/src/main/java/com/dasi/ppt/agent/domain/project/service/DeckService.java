package com.dasi.ppt.agent.domain.project.service;

import com.dasi.ppt.agent.domain.project.model.entity.Constraint;
import com.dasi.ppt.agent.domain.project.model.entity.Deck;
import com.dasi.ppt.agent.domain.project.model.entity.Meta;
import com.dasi.ppt.agent.domain.project.model.entity.Outline;
import com.dasi.ppt.agent.domain.project.model.entity.Slide;
import com.dasi.ppt.agent.domain.project.model.entity.Spec;
import com.dasi.ppt.agent.domain.project.model.enumeration.ChangePart;
import com.dasi.ppt.agent.domain.project.repository.IDeckRepository;
import com.dasi.ppt.agent.domain.project.repository.IProjectRepository;
import com.dasi.ppt.agent.domain.shared.service.PathResolver;
import com.dasi.ppt.agent.domain.shared.exception.ErrorCode;
import com.dasi.ppt.agent.domain.shared.sse.EventPublisher;
import com.dasi.ppt.agent.domain.shared.sse.ProjectChangeArea;
import com.dasi.ppt.agent.domain.shared.sse.SseEvent;
import com.dasi.ppt.agent.domain.shared.util.HashUtil;
import com.dasi.ppt.agent.domain.shared.util.IdUtil;
import com.networknt.schema.Error;
import com.networknt.schema.Schema;
import com.networknt.schema.SchemaRegistry;
import com.networknt.schema.SpecificationVersion;
import lombok.RequiredArgsConstructor;
import org.springframework.core.io.ClassPathResource;
import org.springframework.stereotype.Service;
import org.springframework.transaction.support.TransactionTemplate;
import tools.jackson.core.JacksonException;
import tools.jackson.databind.JsonNode;
import tools.jackson.databind.ObjectMapper;

import java.io.IOException;
import java.io.InputStream;
import java.io.UncheckedIOException;
import java.nio.charset.StandardCharsets;
import java.nio.file.Path;
import java.util.Arrays;
import java.util.EnumMap;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.Objects;

import static com.dasi.ppt.agent.domain.shared.exception.BusinessException.conflict;
import static com.dasi.ppt.agent.domain.shared.exception.BusinessException.illegal;
import static com.dasi.ppt.agent.domain.shared.exception.BusinessException.invalid;
import static com.dasi.ppt.agent.domain.shared.exception.BusinessException.notFound;

@Service
@RequiredArgsConstructor
public class DeckService {

    private final TransactionTemplate transactionTemplate;
    private final ObjectMapper objectMapper;
    private final IProjectRepository projectRepository;
    private final IDeckRepository deckRepository;
    private final PathResolver pathResolver;
    private final ProjectWriteLock projectWriteLock;
    private final EventPublisher eventPublisher;

    private final Map<ChangePart, Schema> schemas = loadSchemas();

    public Deck readDeck(String projectId) {
        if (!projectRepository.existsProject(projectId)) {
            throw notFound(ErrorCode.PROJECT_NOT_FOUND);
        }
        byte[] metaBytes = deckRepository.readFile(pathResolver.metaPath(projectId));
        byte[] constraintBytes = deckRepository.readFile(pathResolver.constraintPath(projectId));
        byte[] outlineBytes = deckRepository.readFile(pathResolver.outlinePath(projectId));

        Meta meta = decode(ChangePart.META, metaBytes, Meta.class);
        Constraint constraint = decode(ChangePart.CONSTRAINT, constraintBytes, Constraint.class);
        Outline outline = decode(ChangePart.OUTLINE, outlineBytes, Outline.class);

        Map<String, Slide> slides = new LinkedHashMap<>();
        if (outline != null) {
            for (String slideId : outline.slideIds()) {
                byte[] spec = deckRepository.readFile(pathResolver.specPath(projectId, slideId));
                byte[] html = deckRepository.readFile(pathResolver.htmlPath(projectId, slideId));
                Slide slide = Slide.builder()
                        .slideId(slideId)
                        .spec(decode(ChangePart.SPEC, spec, Spec.class))
                        .html(html == null ? null : new String(html, StandardCharsets.UTF_8))
                        .specHash(HashUtil.contentHash(spec))
                        .htmlHash(HashUtil.contentHash(html))
                        .build();
                slides.put(slideId, slide);
            }
        }

        return Deck.builder()
                .meta(meta)
                .constraint(constraint)
                .outline(outline)
                .metaHash(HashUtil.contentHash(metaBytes))
                .constraintHash(HashUtil.contentHash(constraintBytes))
                .outlineHash(HashUtil.contentHash(outlineBytes))
                .slides(slides)
                .build();
    }

    public String saveMeta(String projectId, String baseHash, Meta meta) {
        synchronized (projectWriteLock.lockFor(projectId)) {
            if (!projectRepository.existsProject(projectId)) {
                throw notFound(ErrorCode.PROJECT_NOT_FOUND);
            }
            if (projectRepository.hasPendingFileOperation(projectId) || deckRepository.isProjectBusy(projectId)) {
                throw conflict(ErrorCode.PROJECT_BUSY);
            }
            Path path = pathResolver.metaPath(projectId);
            byte[] before = readBase(path, baseHash);
            byte[] after = encode(ChangePart.META, meta);
            return saveDocument(projectId, path, before, after, Map.of());
        }
    }

    public String saveOutline(String projectId, String baseHash, Outline outline) {
        synchronized (projectWriteLock.lockFor(projectId)) {
            if (!projectRepository.existsProject(projectId)) {
                throw notFound(ErrorCode.PROJECT_NOT_FOUND);
            }
            if (projectRepository.hasPendingFileOperation(projectId) || deckRepository.isProjectBusy(projectId)) {
                throw conflict(ErrorCode.PROJECT_BUSY);
            }
            Path path = pathResolver.outlinePath(projectId);
            byte[] before = readBase(path, baseHash);
            byte[] after = encode(ChangePart.OUTLINE, outline);
            validateOutline(outline);
            Map<String, byte[]> orphanFiles = deckRepository.readOrphanSlideFiles(projectId, outline.slideIds());
            return saveDocument(projectId, path, before, after, orphanFiles);
        }
    }

    public String saveConstraint(String projectId, String baseHash, Constraint constraint) {
        synchronized (projectWriteLock.lockFor(projectId)) {
            if (!projectRepository.existsProject(projectId)) {
                throw notFound(ErrorCode.PROJECT_NOT_FOUND);
            }
            if (projectRepository.hasPendingFileOperation(projectId) || deckRepository.isProjectBusy(projectId)) {
                throw conflict(ErrorCode.PROJECT_BUSY);
            }
            Path path = pathResolver.constraintPath(projectId);
            byte[] before = readBase(path, baseHash);
            byte[] after = encode(ChangePart.CONSTRAINT, constraint);
            return saveDocument(projectId, path, before, after, Map.of());
        }
    }

    public String saveSlideSpec(String projectId, String slideId, String baseHash, Spec spec) {
        synchronized (projectWriteLock.lockFor(projectId)) {
            if (!projectRepository.existsProject(projectId)) {
                throw notFound(ErrorCode.PROJECT_NOT_FOUND);
            }
            if (projectRepository.hasPendingFileOperation(projectId) || deckRepository.isProjectBusy(projectId)) {
                throw conflict(ErrorCode.PROJECT_BUSY);
            }
            Path path = pathResolver.specPath(projectId, slideId);
            byte[] before = readBase(path, baseHash);
            byte[] after = encode(ChangePart.SPEC, spec);
            requireSlideInOutline(projectId, slideId);
            return saveDocument(projectId, path, before, after, Map.of());
        }
    }

    public String saveSlideHtml(String projectId, String slideId, String baseHash, String html) {
        synchronized (projectWriteLock.lockFor(projectId)) {
            if (!projectRepository.existsProject(projectId)) {
                throw notFound(ErrorCode.PROJECT_NOT_FOUND);
            }
            if (projectRepository.hasPendingFileOperation(projectId) || deckRepository.isProjectBusy(projectId)) {
                throw conflict(ErrorCode.PROJECT_BUSY);
            }
            Path path = pathResolver.htmlPath(projectId, slideId);
            byte[] before = readBase(path, baseHash);
            byte[] after = html.getBytes(StandardCharsets.UTF_8);
            requireSlideInOutline(projectId, slideId);
            return saveDocument(projectId, path, before, after, Map.of());
        }
    }

    // 需要让所有 slideId 都出现在 outline 里面
    private void requireSlideInOutline(String projectId, String slideId) {
        byte[] outlineBytes = deckRepository.readFile(pathResolver.outlinePath(projectId));
        Outline outline = decode(ChangePart.OUTLINE, outlineBytes, Outline.class);
        if (outline == null || !outline.slideIds().contains(slideId)) {
            throw invalid("幻灯片不在当前大纲中");
        }
    }

    private byte[] readBase(Path path, String baseHash) {
        byte[] before = deckRepository.readFile(path);
        if (!Objects.equals(baseHash, HashUtil.contentHash(before))) {
            throw conflict(ErrorCode.CONTENT_CONFLICT);
        }
        return before;
    }

    // 专属于 outline 的业务逻辑
    private void validateOutline(Outline outline) {
        if (!outline.getSlides().isEmpty() && !outline.getChapters().isEmpty()) {
            throw invalid("大纲根节点不能同时包含直属幻灯片和章");
        }
        for (Outline.Chapter chapter : outline.getChapters()) {
            if (!chapter.getSlides().isEmpty() && !chapter.getSections().isEmpty()) {
                throw invalid("章不能同时包含直属幻灯片和节");
            }
        }
    }

    private String saveDocument(String projectId, Path path, byte[] before, byte[] after, Map<String, byte[]> orphanFiles) {
        String hash = HashUtil.contentHash(after);
        if (orphanFiles.isEmpty() && Arrays.equals(before, after)) return hash;
        String operationId = IdUtil.genOperationId();
        try {
            transactionTemplate.executeWithoutResult(status -> deckRepository.saveManualFiles(projectId, operationId, path, before, after, orphanFiles, System.currentTimeMillis()));
        } catch (RuntimeException ex) {
            // 回滚
            deckRepository.finishManualSave(projectId, operationId, false);
            throw ex;
        }
        // 提交
        deckRepository.finishManualSave(projectId, operationId, true);
        // 发送内容变更通知到会话
        SseEvent.ProjectChanged event = SseEvent.ProjectChanged.builder()
                .areas(List.of(ProjectChangeArea.CONTENT))
                .build();
        projectRepository.listProjectSessionIds(projectId).forEach(id -> eventPublisher.publish(id, event));
        return hash;
    }

    // 启动时加载 meta、constraint、outline、spec 这四份 JSON Schema
    private static Map<ChangePart, Schema> loadSchemas() {
        Map<ChangePart, Schema> schemas = new EnumMap<>(ChangePart.class);
        SchemaRegistry registry = SchemaRegistry.withDefaultDialect(SpecificationVersion.DRAFT_2020_12);
        for (ChangePart part : List.of(ChangePart.META, ChangePart.CONSTRAINT, ChangePart.OUTLINE, ChangePart.SPEC)) {
            String path = "schemas/" + part.getValue() + ".schema.json";
            try (InputStream input = new ClassPathResource(path).getInputStream()) {
                schemas.put(part, registry.getSchema(input));
            } catch (IOException ex) {
                throw new UncheckedIOException(ex);
            }
        }
        return schemas;
    }

    private byte[] encode(ChangePart part, Object document) {
        JsonNode json = objectMapper.valueToTree(document);
        List<Error> errors = schemas.get(part).validate(json);
        if (!errors.isEmpty()) {
            throw invalid(errors.get(0).toString());
        }
        return objectMapper.writeValueAsBytes(json);
    }

    private <T> T decode(ChangePart part, byte[] bytes, Class<T> type) {
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
