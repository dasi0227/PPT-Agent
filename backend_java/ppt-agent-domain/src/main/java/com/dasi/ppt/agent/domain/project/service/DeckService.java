package com.dasi.ppt.agent.domain.project.service;

import com.dasi.ppt.agent.domain.project.model.entity.*;
import com.dasi.ppt.agent.domain.project.model.enumeration.DeckFileType;
import com.dasi.ppt.agent.domain.project.repository.IProjectRepository;
import com.dasi.ppt.agent.domain.setting.model.entity.FileOpenSettings;
import com.dasi.ppt.agent.domain.setting.service.SettingService;
import com.dasi.ppt.agent.domain.shared.exception.ErrorCode;
import com.dasi.ppt.agent.domain.shared.gateway.IFileGateway;
import com.dasi.ppt.agent.domain.shared.gateway.IPlatformGateway;
import com.dasi.ppt.agent.domain.shared.service.NotifyService;
import com.dasi.ppt.agent.domain.shared.service.PathService;
import com.dasi.ppt.agent.domain.shared.service.LockService;
import com.dasi.ppt.agent.domain.shared.service.SchemaService;
import com.dasi.ppt.agent.domain.shared.sse.ProjectChangeArea;
import com.dasi.ppt.agent.domain.shared.util.HashUtil;
import lombok.RequiredArgsConstructor;
import org.springframework.stereotype.Service;
import tools.jackson.databind.ObjectMapper;

import java.nio.charset.StandardCharsets;
import java.nio.file.Path;
import java.util.*;

import static com.dasi.ppt.agent.domain.shared.constant.SystemConstant.*;
import static com.dasi.ppt.agent.domain.shared.exception.BusinessException.*;

@Service
@RequiredArgsConstructor
public class DeckService {

    private final ObjectMapper objectMapper;
    private final SchemaService schemaService;
    private final IProjectRepository projectRepository;
    private final PathService pathService;
    private final LockService lockService;
    private final NotifyService notifyService;
    private final SettingService settingService;
    private final IPlatformGateway platformGateway;
    private final IFileGateway fileGateway;

    // 利用操作系统根据对应的应用程序打开文件
    public void openFile(String projectId, String fileName) {
        // 1. 检查项目是否存在
        if (!projectRepository.existsProject(projectId)) {
            throw notFound(ErrorCode.FILE_NOT_FOUND);
        }
        // 2. 获取设置的应用程序
        FileOpenSettings fileOpenSettings = settingService.getFileOpenSettings();
        String choice = fileName.endsWith(".json") ? fileOpenSettings.getJsonOpen() : fileOpenSettings.getHtmlOpen();
        if (INHERIT_OPEN.equals(choice)) {
            choice = fileOpenSettings.getDefaultOpen();
        }
        Path application = SYSTEM_OPEN.equals(choice) ? null : Path.of(choice);
        // 3. 调用底层打开
        Path path = pathService.resolveDocumentForOpen(projectId, fileName);
        platformGateway.openFile(path, application);
    }

    public Deck readDeck(String projectId) {
        if (!projectRepository.existsProject(projectId)) {
            throw notFound(ErrorCode.PROJECT_NOT_FOUND);
        }

        Path projectDirectory = pathService.projectDirectory(projectId);
        byte[] metaBytes = fileGateway.readFile(projectDirectory.resolve(META_FILE_NAME));
        byte[] constraintBytes = fileGateway.readFile(projectDirectory.resolve(CONSTRAINT_FILE_NAME));
        byte[] outlineBytes = fileGateway.readFile(projectDirectory.resolve(OUTLINE_FILE_NAME));

        Meta meta = schemaService.decode(DeckFileType.META, metaBytes, Meta.class);
        Constraint constraint = schemaService.decode(DeckFileType.CONSTRAINT, constraintBytes, Constraint.class);
        Outline outline = schemaService.decode(DeckFileType.OUTLINE, outlineBytes, Outline.class);

        List<Slide> slides = new ArrayList<>();
        if (outline != null) {
            for (String slideId : outline.slideIds()) {
                byte[] spec = fileGateway.readFile(projectDirectory.resolve(SLIDE_SPEC_FILE_NAME_FORMAT.formatted(slideId)));
                byte[] html = fileGateway.readFile(projectDirectory.resolve(SLIDE_HTML_FILE_NAME_FORMAT.formatted(slideId)));
                Slide slide = Slide.builder()
                        .slideId(slideId)
                        .spec(schemaService.decode(DeckFileType.SPEC, spec, Spec.class))
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
        // 1. 根据文件类型确定文稿文件名
        String documentFileName = switch (fileType) {
            case META -> META_FILE_NAME;
            case CONSTRAINT -> CONSTRAINT_FILE_NAME;
            case OUTLINE -> OUTLINE_FILE_NAME;
            case SPEC -> SLIDE_SPEC_FILE_NAME_FORMAT.formatted(slideId);
            case HTML -> SLIDE_HTML_FILE_NAME_FORMAT.formatted(slideId);
        };
        String hash;

        // 2. 对项目上锁
        synchronized (lockService.projectLock(projectId)) {
            // 3.1 校验项目是否存在
            if (!projectRepository.existsProject(projectId)) {
                throw notFound(ErrorCode.PROJECT_NOT_FOUND);
            }
            // 3.2 校验项目是否正在运行中
            if (projectRepository.hasPendingFileOperation(projectId) || projectRepository.isProjectBusy(projectId)) {
                throw conflict(ErrorCode.PROJECT_BUSY);
            }
            // 3.3 校验当前文件内容与哈希一致
            Path projectDirectory = pathService.projectDirectory(projectId);
            byte[] before = fileGateway.readFile(projectDirectory.resolve(documentFileName));
            if (!Objects.equals(baseHash, HashUtil.contentHash(before))) {
                throw conflict(ErrorCode.CONTENT_CONFLICT);
            }
            // 3.4 校验 JSON 文件格式
            if (fileType != DeckFileType.HTML) {
                schemaService.validate(fileType, content);
            }
            // 3.5 校验 slideId 真实存在
            if (fileType == DeckFileType.SPEC || fileType == DeckFileType.HTML) {
                byte[] outlineBytes = fileGateway.readFile(projectDirectory.resolve(OUTLINE_FILE_NAME));
                Outline currentOutline = schemaService.decode(DeckFileType.OUTLINE, outlineBytes, Outline.class);
                if (currentOutline == null || !currentOutline.slideIds().contains(slideId)) {
                    throw invalid("幻灯片不在当前大纲中");
                }
            }
            // 3.6 校验 outline 符合格式
            if (outline != null) {
                if (!outline.getPages().isEmpty() && !outline.getChapters().isEmpty()) {
                    throw invalid("大纲根节点不能同时包含直属幻灯片和章");
                }
                for (Outline.Chapter chapter : outline.getChapters()) {
                    if (!chapter.getPages().isEmpty() && !chapter.getSections().isEmpty()) {
                        throw invalid("章不能同时包含直属幻灯片和节");
                    }
                }
            }

            // 4. 记录文件变更
            hash = HashUtil.contentHash(content);
            List<FileChange> fileChanges = new ArrayList<>();
            if (!Arrays.equals(before, content)) {
                FileChange fileChange = FileChange.builder()
                        .fileName(documentFileName)
                        .before(before)
                        .after(content)
                        .build();
                fileChanges.add(fileChange);
            }

            // 5. 记录被删除的页面
            if (outline != null) {
                Set<String> slideIds = outline.slideIds();
                List<String> fileNames = fileGateway.listFileNames(projectDirectory)
                        .stream()
                        .sorted()
                        .toList();
                for (String fileName : fileNames) {
                    String id = pathService.slideIdFromFileName(fileName);
                    if (id != null && !slideIds.contains(id)) {
                        FileChange fileChange = FileChange.builder()
                                .fileName(fileName)
                                .before(fileGateway.readFile(projectDirectory.resolve(fileName)))
                                .after(null)
                                .build();
                        fileChanges.add(fileChange);
                    }
                }
            }

            // 6. 执行变更
            if (fileChanges.isEmpty()) {
                return hash;
            }
            projectRepository.commitFiles(projectId, fileChanges);
        }

        // 7. 通知项目
        notifyService.notifyProjectChanged(projectId, ProjectChangeArea.CONTENT);
        return hash;
    }
}
