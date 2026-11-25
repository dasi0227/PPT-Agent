package com.dasi.ppt.agent.infrastructure.repository;

import com.dasi.ppt.agent.domain.project.model.entity.Constraint;
import com.dasi.ppt.agent.domain.project.model.entity.Deck;
import com.dasi.ppt.agent.domain.project.model.entity.Meta;
import com.dasi.ppt.agent.domain.project.model.entity.Outline;
import com.dasi.ppt.agent.domain.project.model.entity.Slide;
import com.dasi.ppt.agent.domain.project.model.entity.Spec;
import com.dasi.ppt.agent.domain.project.model.enumeration.ChangePart;
import com.dasi.ppt.agent.domain.project.repository.IDeckRepository;
import com.dasi.ppt.agent.domain.project.service.DeckDocuments;
import com.dasi.ppt.agent.domain.shared.util.HashUtil;
import com.dasi.ppt.agent.infrastructure.persistence.database.mapper.ProjectMapper;
import com.dasi.ppt.agent.infrastructure.persistence.database.mapper.ReceiptMapper;
import com.dasi.ppt.agent.infrastructure.persistence.database.po.ReceiptPo;
import com.dasi.ppt.agent.infrastructure.persistence.filesystem.FileStorer;
import com.dasi.ppt.agent.infrastructure.persistence.filesystem.MutationLog;
import com.dasi.ppt.agent.infrastructure.persistence.filesystem.MutationLogType;
import com.dasi.ppt.agent.infrastructure.persistence.filesystem.PathResolver;
import lombok.RequiredArgsConstructor;
import lombok.extern.slf4j.Slf4j;
import org.springframework.beans.factory.SmartInitializingSingleton;
import org.springframework.stereotype.Repository;
import tools.jackson.core.JacksonException;
import tools.jackson.databind.ObjectMapper;

import java.io.IOException;
import java.io.UncheckedIOException;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.NoSuchFileException;
import java.nio.file.Path;
import java.util.ArrayList;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.Set;

import static com.dasi.ppt.agent.domain.shared.constant.SystemConstant.CONSTRAINT_FILE_NAME;
import static com.dasi.ppt.agent.domain.shared.constant.SystemConstant.META_FILE_NAME;
import static com.dasi.ppt.agent.domain.shared.constant.SystemConstant.OUTLINE_FILE_NAME;
import static com.dasi.ppt.agent.domain.shared.exception.BusinessException.illegal;
@Slf4j
@Repository
@RequiredArgsConstructor
public class DeckRepository implements IDeckRepository, SmartInitializingSingleton {

    private final DeckDocuments deckDocuments;
    private final FileStorer fileStorer;
    private final PathResolver pathResolver;
    private final ReceiptMapper receiptMapper;
    private final ProjectMapper projectMapper;
    private final ObjectMapper objectMapper;

    @Override
    public Deck readDeck(String projectId) {
        Path directory = pathResolver.projectDirectory(projectId);
        byte[] metaBytes = fileStorer.readFile(directory.resolve(META_FILE_NAME));
        byte[] constraintBytes = fileStorer.readFile(directory.resolve(CONSTRAINT_FILE_NAME));
        byte[] outlineBytes = fileStorer.readFile(directory.resolve(OUTLINE_FILE_NAME));

        Meta meta = deckDocuments.decode(ChangePart.META, metaBytes, Meta.class);
        Constraint constraint = deckDocuments.decode(ChangePart.CONSTRAINT, constraintBytes, Constraint.class);
        Outline outline = deckDocuments.decode(ChangePart.OUTLINE, outlineBytes, Outline.class);

        Map<String, Slide> slides = new LinkedHashMap<>();
        if (outline != null) {
            for (String slideId : outline.slideIds()) {
                byte[] spec = fileStorer.readFile(directory.resolve(slideId + ".json"));
                byte[] html = fileStorer.readFile(directory.resolve(slideId + ".html"));
                Slide slide = Slide.builder()
                        .slideId(slideId)
                        .spec(deckDocuments.decode(ChangePart.SPEC, spec, Spec.class))
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

    @Override
    public byte[] readFile(String projectId, String fileName) {
        return fileStorer.readFile(pathResolver.projectDirectory(projectId).resolve(fileName));
    }

    @Override
    public boolean isProjectBusy(String projectId) {
        return projectMapper.hasActiveDocumentOperation(projectId);
    }

    @Override
    public void saveManualFiles(String projectId, String operationId, String path, byte[] before,
                                byte[] after, Set<String> removedSlides, long updatedAt) {
        Path directory = pathResolver.projectDirectory(projectId);
        List<MutationLog.FileChange> files = new ArrayList<>();
        files.add(MutationLog.FileChange.builder()
                .path(path)
                .before(before)
                .after(after)
                .build());
        for (String id : removedSlides) {
            for (String extension : List.of(".json", ".html")) {
                String name = id + extension;
                byte[] old = fileStorer.readFile(directory.resolve(name));
                if (old != null) {
                    files.add(MutationLog.FileChange.builder()
                            .path(name)
                            .before(old)
                            .build());
                }
            }
        }
        MutationLog mutation = MutationLog.builder()
                .type(MutationLogType.MANUAL)
                .ownerId(projectId)
                .requestId(operationId)
                .requestHash(HashUtil.sha256(objectMapper.writeValueAsBytes(files)))
                .files(files)
                .build();
        fileStorer.writeMutationLog(manualLogPath(projectId, operationId), objectMapper.writeValueAsBytes(mutation));
        for (MutationLog.FileChange file : files) {
            fileStorer.replaceFile(directory.resolve(file.getPath()), file.getAfter());
        }
        receiptMapper.insertManualReceipt(ReceiptPo.builder()
                .type(mutation.getType().getValue())
                .ownerId(projectId)
                .requestId(operationId)
                .requestHash(mutation.getRequestHash())
                .build());
        projectMapper.updateProjectTime(projectId, updatedAt);
    }

    private Path manualLogPath(String projectId, String operationId) {
        String key = "manual\0" + projectId + "\0" + operationId;
        return fileStorer.mutationLogPath(pathResolver.projectDirectory(projectId), HashUtil.sha256(key.getBytes(StandardCharsets.UTF_8)));
    }

    @Override
    public void finishManualSave(String projectId, String operationId, boolean committed) {
        Path path = manualLogPath(projectId, operationId);
        if (committed) {
            fileStorer.replaceFile(path, null);
            receiptMapper.deleteManualReceipt(projectId, operationId);
        } else {
            recoverManualLog(projectId, path);
        }
    }

    @Override
    public void afterSingletonsInstantiated() {
        // 项目创建、删除的目录恢复已在 ProjectRepository 初始化时完成。
        try (var projects = Files.newDirectoryStream(pathResolver.projectsRoot())) {
            for (Path directory : projects) {
                if (!Files.isDirectory(directory) || directory.getFileName().toString().startsWith(".")) continue;
                try {
                    for (Path path : fileStorer.listMutationLogs(directory)) {
                        recoverManualLog(directory.getFileName().toString(), path);
                    }
                } catch (RuntimeException ex) {
                    log.error("文稿恢复失败，项目保留写入日志：{}", directory, ex);
                }
            }
        } catch (NoSuchFileException ignored) {
            // 尚未创建项目。
        } catch (IOException ex) {
            throw new UncheckedIOException(ex);
        }
    }

    private void recoverManualLog(String projectId, Path path) {
        byte[] bytes = fileStorer.readFile(path);
        if (bytes == null) return;
        MutationLog mutation;
        try {
            mutation = objectMapper.readValue(bytes, MutationLog.class);
        } catch (JacksonException ex) {
            throw illegal("文稿日志格式无效：" + path);
        }
        if (mutation.getType() != MutationLogType.MANUAL) return;
        String operationId = mutation.getRequestId();
        if (!projectId.equals(mutation.getOwnerId()) || !path.equals(manualLogPath(projectId, operationId))
                || !HashUtil.sha256(objectMapper.writeValueAsBytes(mutation.getFiles())).equals(mutation.getRequestHash())) {
            throw illegal("文稿日志与操作摘要不符：" + path);
        }
        String hash = receiptMapper.findManualRequestHash(projectId, operationId);
        if (hash != null && !hash.equals(mutation.getRequestHash())) {
            throw illegal("文稿日志与提交凭据不符：" + path);
        }
        for (MutationLog.FileChange change : mutation.getFiles()) {
            if (!change.getPath().matches("\\.(meta|constraint|outline)\\.json|sli_[A-Za-z0-9_-]+\\.(json|html)")) {
                throw illegal("文稿日志路径无效：" + change.getPath());
            }
            byte[] target = hash == null ? change.getBefore() : change.getAfter();
            fileStorer.replaceFile(pathResolver.projectDirectory(projectId).resolve(change.getPath()), target);
        }
        fileStorer.replaceFile(path, null);
        if (hash != null) receiptMapper.deleteManualReceipt(projectId, operationId);
    }
}
