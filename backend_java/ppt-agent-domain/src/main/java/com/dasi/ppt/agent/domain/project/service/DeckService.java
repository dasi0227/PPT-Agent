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

import java.io.IOException;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.NoSuchFileException;
import java.nio.file.Path;
import java.nio.file.attribute.BasicFileAttributes;
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
        Path path = resolveDocumentForOpen(projectId, fileName);
        platformGateway.openFile(path, application);
    }

    private Path resolveDocumentForOpen(String projectId, String fileName) {
        try {
            Path directory = pathService.projectsRoot().resolve(projectId);
            Path path = directory.resolve(fileName);
            if (!pathService.isProjectFile(directory, path) || !Files.readAttributes(path, BasicFileAttributes.class).isRegularFile()) {
                throw invalid("只能打开项目根目录下的普通文稿文件");
            }
            return path;
        } catch (NoSuchFileException e) {
            throw notFound(ErrorCode.FILE_NOT_FOUND);
        } catch (IOException e) {
            throw failed(ErrorCode.FILE_OPEN_FAILED, e.getMessage());
        }
    }

    public Deck readDeck(String projectId) {
        if (!projectRepository.existsProject(projectId)) {
            throw notFound(ErrorCode.PROJECT_NOT_FOUND);
        }

        byte[] metaBytes = fileGateway.readFile(pathService.meta(projectId));
        byte[] constraintBytes = fileGateway.readFile(pathService.constraint(projectId));
        byte[] outlineBytes = fileGateway.readFile(pathService.outline(projectId));

        Meta meta = schemaService.decode(DeckFileType.META, metaBytes, Meta.class);
        Constraint constraint = schemaService.decode(DeckFileType.CONSTRAINT, constraintBytes, Constraint.class);
        Outline outline = schemaService.decode(DeckFileType.OUTLINE, outlineBytes, Outline.class);

        List<Slide> slides = new ArrayList<>();
        if (outline != null) {
            for (String slideId : outline.slideIds()) {
                byte[] spec = fileGateway.readFile(pathService.slideSpec(projectId, slideId));
                byte[] html = fileGateway.readFile(pathService.slideHtml(projectId, slideId));
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
        String hash;
        // 1. 对项目上锁
        synchronized (lockService.projectLock(projectId)) {
            // 2.1 校验项目是否存在
            if (!projectRepository.existsProject(projectId)) {
                throw notFound(ErrorCode.PROJECT_NOT_FOUND);
            }
            // 2.2 校验项目是否正在运行中
            if (projectRepository.hasPendingFileOperation(projectId) || projectRepository.isProjectBusy(projectId)) {
                throw conflict(ErrorCode.PROJECT_BUSY);
            }
            // 2.3 校验当前文件内容与哈希一致
            Path documentFile = switch (fileType) {
                case META -> pathService.meta(projectId);
                case CONSTRAINT -> pathService.constraint(projectId);
                case OUTLINE -> pathService.outline(projectId);
                case SPEC -> pathService.slideSpec(projectId, slideId);
                case HTML -> pathService.slideHtml(projectId, slideId);
            };
            byte[] before = fileGateway.readFile(documentFile);
            if (!Objects.equals(baseHash, HashUtil.contentHash(before))) {
                throw conflict(ErrorCode.CONTENT_CONFLICT);
            }
            // 2.4 校验 JSON 文件格式
            if (fileType != DeckFileType.HTML) {
                schemaService.validate(fileType, content);
            }
            // 2.5 校验 slideId 真实存在
            if (fileType == DeckFileType.SPEC || fileType == DeckFileType.HTML) {
                byte[] outlineBytes = fileGateway.readFile(pathService.outline(projectId));
                Outline currentOutline = schemaService.decode(DeckFileType.OUTLINE, outlineBytes, Outline.class);
                if (currentOutline == null || !currentOutline.slideIds().contains(slideId)) {
                    throw invalid("幻灯片不在当前大纲中");
                }
            }
            // 2.6 校验 outline 符合格式
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

            // 3. 记录文件变更
            hash = HashUtil.contentHash(content);
            List<FileChange> fileChanges = new ArrayList<>();
            if (!Arrays.equals(before, content)) {
                FileChange fileChange = FileChange.builder()
                        .fileName(documentFile.getFileName().toString())
                        .before(before)
                        .after(content)
                        .build();
                fileChanges.add(fileChange);
            }

            // 4. 记录被删除的页面
            if (outline != null) {
                Set<String> slideIds = outline.slideIds();
                Path projectDirectory = pathService.projectDirectory(projectId);
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

            // 5. 执行变更
            if (fileChanges.isEmpty()) {
                return hash;
            }
            projectRepository.commitFiles(projectId, fileChanges);
        }

        // 6. 通知项目
        notifyService.notifyProjectChanged(projectId, ProjectChangeArea.CONTENT);
        return hash;
    }
}
