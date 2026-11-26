package com.dasi.ppt.agent.domain.project.service;

import com.dasi.ppt.agent.domain.asset.service.AssetService;
import com.dasi.ppt.agent.domain.project.model.entity.*;
import com.dasi.ppt.agent.domain.project.model.enumeration.DeckFileType;
import com.dasi.ppt.agent.domain.project.model.enumeration.DecorationPosition;
import com.dasi.ppt.agent.domain.project.model.enumeration.DecorationType;
import com.dasi.ppt.agent.domain.project.repository.IProjectRepository;
import com.dasi.ppt.agent.domain.shared.exception.ErrorCode;
import com.dasi.ppt.agent.domain.shared.service.PathResolver;
import com.dasi.ppt.agent.domain.shared.sse.EventPublisher;
import com.dasi.ppt.agent.domain.shared.sse.ProjectChangeArea;
import com.dasi.ppt.agent.domain.shared.sse.SseEvent;
import com.dasi.ppt.agent.domain.shared.util.HashUtil;
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
import java.util.*;
import java.util.function.Consumer;

import static com.dasi.ppt.agent.domain.shared.exception.BusinessException.*;

@Service
@RequiredArgsConstructor
public class DeckService {

    private final TransactionTemplate transactionTemplate;
    private final ObjectMapper objectMapper;
    private final IProjectRepository projectRepository;
    private final PathResolver pathResolver;
    private final ProjectWriteLock projectWriteLock;
    private final EventPublisher eventPublisher;
    private final AssetService assetService;

    private final Map<DeckFileType, Schema> schemas = loadSchemas();

    // 启动时加载 meta、constraint、outline、spec 这四份 JSON Schema
    private static Map<DeckFileType, Schema> loadSchemas() {
        Map<DeckFileType, Schema> schemas = new EnumMap<>(DeckFileType.class);
        SchemaRegistry registry = SchemaRegistry.withDefaultDialect(SpecificationVersion.DRAFT_2020_12);
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

    private byte[] readAndCheckBase(Path path, String baseHash) {
        byte[] before = projectRepository.readFile(path);
        if (!Objects.equals(baseHash, HashUtil.contentHash(before))) {
            throw conflict(ErrorCode.CONTENT_CONFLICT);
        }
        return before;
    }

    public Deck readDeck(String projectId) {
        if (!projectRepository.existsProject(projectId)) {
            throw notFound(ErrorCode.PROJECT_NOT_FOUND);
        }

        byte[] metaBytes = projectRepository.readFile(pathResolver.metaPath(projectId));
        byte[] constraintBytes = projectRepository.readFile(pathResolver.constraintPath(projectId));
        byte[] outlineBytes = projectRepository.readFile(pathResolver.outlinePath(projectId));

        Meta meta = decode(DeckFileType.META, metaBytes, Meta.class);
        Constraint constraint = decode(DeckFileType.CONSTRAINT, constraintBytes, Constraint.class);
        Outline outline = decode(DeckFileType.OUTLINE, outlineBytes, Outline.class);

        List<Slide> slides = new ArrayList<>();
        if (outline != null) {
            for (String slideId : outline.slideIds()) {
                byte[] spec = projectRepository.readFile(pathResolver.specPath(projectId, slideId));
                byte[] html = projectRepository.readFile(pathResolver.htmlPath(projectId, slideId));
                Slide slide = Slide.builder()
                        .slideId(slideId)
                        .spec(decode(DeckFileType.SPEC, spec, Spec.class))
                        .html(html == null ? null : new String(html, StandardCharsets.UTF_8))
                        .specHash(HashUtil.contentHash(spec))
                        .htmlHash(HashUtil.contentHash(html))
                        .build();
                slides.add(slide);
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
        byte[] content = objectMapper.writeValueAsBytes(meta);
        return saveDocument(projectId, baseHash, content, DeckFileType.META, null, null);
    }

    public String saveOutline(String projectId, String baseHash, Outline outline) {
        byte[] content = objectMapper.writeValueAsBytes(outline);
        return saveDocument(projectId, baseHash, content, DeckFileType.OUTLINE, null, outline);
    }

    public String saveConstraint(String projectId, String baseHash, Constraint constraint) {
        byte[] content = objectMapper.writeValueAsBytes(constraint);
        return saveDocument(projectId, baseHash, content, DeckFileType.CONSTRAINT, null, null);
    }

    public String saveSlideSpec(String projectId, String slideId, String baseHash, Spec spec) {
        byte[] content = objectMapper.writeValueAsBytes(spec);
        return saveDocument(projectId, baseHash, content, DeckFileType.SPEC, slideId, null);
    }

    public String saveSlideHtml(String projectId, String slideId, String baseHash, String html) {
        byte[] content = html.getBytes(StandardCharsets.UTF_8);
        return saveDocument(projectId, baseHash, content, DeckFileType.HTML, slideId, null);
    }

    private String saveDocument(String projectId, String baseHash, byte[] content, DeckFileType fileType, String slideId, Outline outline) {
        // 1. 根据文件类型解析得到文件路径
        Path path = switch (fileType) {
            case META -> pathResolver.metaPath(projectId);
            case CONSTRAINT -> pathResolver.constraintPath(projectId);
            case OUTLINE -> pathResolver.outlinePath(projectId);
            case SPEC -> pathResolver.specPath(projectId, slideId);
            case HTML -> pathResolver.htmlPath(projectId, slideId);
        };
        String hash;

        // 2. 对项目上锁
        synchronized (projectWriteLock.lockFor(projectId)) {
            // 3.1 校验项目是否存在
            if (!projectRepository.existsProject(projectId)) {
                throw notFound(ErrorCode.PROJECT_NOT_FOUND);
            }
            // 3.2 校验项目是否正在运行中
            if (projectRepository.hasPendingFileOperation(projectId) || projectRepository.isProjectBusy(projectId)) {
                throw conflict(ErrorCode.PROJECT_BUSY);
            }
            byte[] before = readAndCheckBase(path, baseHash);
            if (fileType != DeckFileType.HTML) {
                List<Error> errors = schemas.get(fileType).validate(objectMapper.readTree(content));
                if (!errors.isEmpty()) throw invalid(errors.get(0).toString());
            }
            if (fileType == DeckFileType.SPEC || fileType == DeckFileType.HTML) {
                requireSlideInOutline(projectId, slideId);
            }
            hash = HashUtil.contentHash(content);
            List<FileChange> changes = new ArrayList<>();
            addChange(changes, path.getFileName().toString(), before, content);
            if (outline != null) {
                validateOutline(outline);
                Set<String> slideIds = outline.slideIds();
                for (String name : projectRepository.listDocumentFileNames(projectId)) {
                    String fileSlideId = pathResolver.slideIdFromFileName(name);
                    if (fileSlideId != null && !slideIds.contains(fileSlideId)) {
                        addChange(changes, name, projectRepository.readFile(pathResolver.filePathByFileName(projectId, name)), null);
                    }
                }
            }
            if (changes.isEmpty()) return hash;
            projectRepository.commitFiles(projectId, changes, () -> projectRepository.updateProjectTime(projectId, System.currentTimeMillis()));
        }
        notifyContentChanged(projectId);
        return hash;
    }

    // 需要让所有 slideId 都出现在 outline 里面
    private void requireSlideInOutline(String projectId, String slideId) {
        byte[] outlineBytes = projectRepository.readFile(pathResolver.outlinePath(projectId));
        Outline outline = decode(DeckFileType.OUTLINE, outlineBytes, Outline.class);
        if (outline == null || !outline.slideIds().contains(slideId)) {
            throw invalid("幻灯片不在当前大纲中");
        }
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

    public void notifyContentChanged(String projectId) {
        SseEvent.ProjectChanged event = SseEvent.ProjectChanged.builder()
                .areas(List.of(ProjectChangeArea.CONTENT))
                .build();
        projectRepository.listProjectSessionIds(projectId).forEach(id -> eventPublisher.publish(id, event));
    }

    // 调用方在同一项目锁内复核授权或检查点；保存业务结果的回调在最终事务中执行。
    public boolean commitDocumentFiles(String projectId, Map<String, byte[]> replacements, Map<String, String> baseHashes, Consumer<List<FileChange>> saveBusiness) {
        List<FileChange> changes = new ArrayList<>();
        replacements.forEach((name, after) -> {
            if (!pathResolver.isDocumentFileName(name)) throw invalid("无效的文稿文件名：" + name);
            Path path = pathResolver.filePathByFileName(projectId, name);
            byte[] before = baseHashes.containsKey(name) ? readAndCheckBase(path, baseHashes.get(name)) : projectRepository.readFile(path);
            addChange(changes, name, before, after);
        });
        projectRepository.commitFiles(projectId, changes, () -> saveBusiness.accept(changes));
        return !changes.isEmpty();
    }

    private void addChange(List<FileChange> changes, String name, byte[] before, byte[] after) {
        if (!Arrays.equals(before, after)) {
            changes.add(FileChange.builder()
                    .fileName(name)
                    .before(before)
                    .after(after)
                    .build());
        }
    }

    private <T> T decode(DeckFileType part, byte[] bytes, Class<T> type) {
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


    // 调用方持有项目锁。缩略图只读取目标页源码，同时保留完整页序以确定页码。
    public ExportSnapshot readRenderSnapshot(String projectId, String targetSlideId) {
        return transactionTemplate.execute(status -> {
            Project project = projectRepository.findProjectAppearance(projectId);
            if (project == null) return null;
            Meta meta = decode(DeckFileType.META, projectRepository.readFile(pathResolver.metaPath(projectId)), Meta.class);
            Outline outline = decode(DeckFileType.OUTLINE, projectRepository.readFile(pathResolver.outlinePath(projectId)), Outline.class);
            List<ExportSnapshot.Page> pages = new ArrayList<>();
            if (outline != null) {
                validateOutline(outline);
                outline.forEachPage((page, chapterTitle) -> {
                    byte[] html = targetSlideId == null || targetSlideId.equals(page.getId())
                            ? projectRepository.readFile(pathResolver.htmlPath(projectId, page.getId())) : null;
                    pages.add(ExportSnapshot.Page.builder()
                            .slideId(page.getId())
                            .title(page.getTitle())
                            .chapterTitle(chapterTitle)
                            .html(html == null ? null : new String(html, StandardCharsets.UTF_8))
                            .build());
                });
            }
            Map<DecorationType, DecorationPosition> positions = new EnumMap<>(DecorationType.class);
            positions.put(DecorationType.SLIDE_PAGINATION, project.getSlidePaginationPosition());
            positions.put(DecorationType.DECK_TITLE, project.getDeckTitlePosition());
            positions.put(DecorationType.CHAPTER_TITLE, project.getChapterTitlePosition());
            positions.put(DecorationType.LOGO, project.getLogoPosition());
            return ExportSnapshot.builder()
                    .projectName(project.getTitle())
                    .deckTitle(meta == null ? null : meta.getTitle())
                    .pages(List.copyOf(pages))
                    .themeCss(project.getTheme() == null ? null : assetService.getAssetDetail(project.getTheme()).getContent())
                    .positions(Collections.unmodifiableMap(positions))
                    .logo(project.isLogoPresent() ? projectRepository.findProjectLogo(projectId) : null)
                    .build();
        });
    }
}
