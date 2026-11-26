package com.dasi.ppt.agent.infrastructure.repository;

import com.dasi.ppt.agent.domain.project.repository.IDeckRepository;
import com.dasi.ppt.agent.domain.shared.service.PathResolver;
import com.dasi.ppt.agent.domain.shared.util.HashUtil;
import com.dasi.ppt.agent.infrastructure.persistence.database.mapper.ProjectMapper;
import com.dasi.ppt.agent.infrastructure.persistence.database.mapper.ReceiptMapper;
import com.dasi.ppt.agent.infrastructure.persistence.database.po.ReceiptPo;
import com.dasi.ppt.agent.infrastructure.persistence.filesystem.FileStorer;
import com.dasi.ppt.agent.infrastructure.persistence.filesystem.model.FileChange;
import com.dasi.ppt.agent.infrastructure.persistence.filesystem.model.Mutation;
import com.dasi.ppt.agent.infrastructure.persistence.filesystem.model.MutationType;
import lombok.RequiredArgsConstructor;
import lombok.extern.slf4j.Slf4j;
import org.springframework.beans.factory.SmartInitializingSingleton;
import org.springframework.stereotype.Repository;
import tools.jackson.core.JacksonException;
import tools.jackson.databind.ObjectMapper;

import java.io.IOException;
import java.io.UncheckedIOException;
import java.nio.file.DirectoryStream;
import java.nio.file.Files;
import java.nio.file.NoSuchFileException;
import java.nio.file.Path;
import java.util.*;

import static com.dasi.ppt.agent.domain.shared.exception.BusinessException.illegal;
import static java.nio.file.LinkOption.NOFOLLOW_LINKS;

@Slf4j
@Repository
@RequiredArgsConstructor
public class DeckRepository implements IDeckRepository, SmartInitializingSingleton {

    private final FileStorer fileStorer;
    private final PathResolver pathResolver;
    private final ReceiptMapper receiptMapper;
    private final ProjectMapper projectMapper;
    private final ObjectMapper objectMapper;

    @Override
    public void afterSingletonsInstantiated() {
        // 项目创建、删除的目录恢复已在 ProjectRepository 初始化时完成。
        try (DirectoryStream<Path> projects = Files.newDirectoryStream(pathResolver.projectsRoot())) {
            for (Path directory : projects) {
                if (!Files.isDirectory(directory) || directory.getFileName().toString().startsWith(".")) continue;
                String projectId = directory.getFileName().toString();
                try {
                    for (Path path : fileStorer.listMutationLogs(projectId)) {
                        recoverManualMutation(projectId, path);
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

    @Override
    public byte[] readFile(Path path) {
        return fileStorer.readFile(path);
    }

    @Override
    public Map<String, byte[]> readOrphanSlideFiles(String projectId, Set<String> slideIds) {
        Path directory = pathResolver.projectDirectory(projectId);
        Map<String, byte[]> orphanFiles = new LinkedHashMap<>();
        try (DirectoryStream<Path> entries = Files.newDirectoryStream(directory)) {
            for (Path file : entries) {
                String name = file.getFileName().toString();
                String slideId = pathResolver.slideIdFromFileName(name);
                if (slideId != null && !slideIds.contains(slideId) && Files.isRegularFile(file, NOFOLLOW_LINKS)) {
                    byte[] before = fileStorer.readFile(file);
                    if (before != null) orphanFiles.put(name, before);
                }
            }
        } catch (IOException ex) {
            throw new UncheckedIOException(ex);
        }
        return orphanFiles;
    }

    @Override
    public boolean isProjectBusy(String projectId) {
        return projectMapper.hasActiveDocumentOperation(projectId);
    }

    @Override
    public void saveManualFiles(String projectId, String operationId, Path path, byte[] before, byte[] after, Map<String, byte[]> orphanFiles, long updatedAt) {
        List<FileChange> fileChanges = new ArrayList<>();
        // 把当前的更改加进去
        if (!Arrays.equals(before, after)) {
            FileChange fileChange = FileChange.builder()
                    .fileName(path.getFileName().toString())
                    .before(before)
                    .after(after)
                    .build();
            fileChanges.add(fileChange);
        }
        // 把当前的删除加进去
        orphanFiles.forEach((name, bytes) -> {
            FileChange fileChange = FileChange.builder()
                    .fileName(name)
                    .before(bytes)
                    .build();
            fileChanges.add(fileChange);
        });
        // 构造一个 mutation 对象，记录此次操作的所有文件变动
        Mutation mutation = Mutation.builder()
                .type(MutationType.MANUAL)
                .ownerId(projectId)
                .requestId(operationId)
                .requestHash(HashUtil.contentHash(objectMapper.writeValueAsBytes(fileChanges)))
                .fileChanges(fileChanges)
                .build();
        fileStorer.writeMutationLog(pathResolver.mutationPath(projectId, operationId), objectMapper.writeValueAsBytes(mutation));
        for (FileChange fileChange : fileChanges) {
            fileStorer.applyFileChange(pathResolver.filePathByFileName(projectId, fileChange.getFileName()), fileChange.getAfter());
        }
        ReceiptPo receiptPo = ReceiptPo.builder()
                .type(mutation.getType().getValue())
                .ownerId(projectId)
                .requestId(operationId)
                .requestHash(mutation.getRequestHash())
                .build();
        receiptMapper.insertReceipt(receiptPo);
        projectMapper.updateProjectTime(projectId, updatedAt);
    }

    @Override
    public void finishManualSave(String projectId, String operationId, boolean committed) {
        Path mutationPath = pathResolver.mutationPath(projectId, operationId);
        if (committed) {
            fileStorer.applyFileChange(mutationPath, null);
            receiptMapper.deleteManualReceipt(projectId, operationId);
        } else {
            recoverManualMutation(projectId, mutationPath);
        }
    }

    private void recoverManualMutation(String projectId, Path mutatationPath) {
        // 先读取变更日志
        byte[] bytes = fileStorer.readFile(mutatationPath);
        if (bytes == null) return;
        Mutation mutation;
        try {
            mutation = objectMapper.readValue(bytes, Mutation.class);
        } catch (JacksonException ex) {
            throw illegal("文稿日志格式无效：" + mutatationPath);
        }
        if (mutation.getType() != MutationType.MANUAL)
            return;
        // 判断该日志是否有效
        String operationId = mutation.getRequestId();
        if (!projectId.equals(mutation.getOwnerId())) {
            throw illegal("文稿日志的项目 ID 与当前项目不符：" + mutatationPath);
        }
        if (!mutatationPath.getFileName().toString().equals(operationId + ".json")) {
            throw illegal("文稿日志文件名与操作 ID 不符：" + mutatationPath);
        }
        String actualHash = HashUtil.contentHash(objectMapper.writeValueAsBytes(mutation.getFileChanges()));
        if (!actualHash.equals(mutation.getRequestHash())) {
            throw illegal("文稿日志的文件变更摘要与记录值不符：" + mutatationPath);
        }
        String requestHash = receiptMapper.findManualRequestHash(projectId, operationId);
        if (requestHash != null && !requestHash.equals(mutation.getRequestHash())) {
            throw illegal("文稿日志与提交凭据不符：" + mutatationPath);
        }
        for (FileChange fileChange : mutation.getFileChanges()) {
            String fileName = fileChange.getFileName();
            if (!pathResolver.isDocumentFileName(fileName)) {
                throw illegal("文稿日志路径无效：" + fileName);
            }
        }
        // 判断是否提交
        boolean committed = requestHash != null;
        for (FileChange fileChange : mutation.getFileChanges()) {
            // 根据提交情况回滚或恢复
            Path path = pathResolver.filePathByFileName(projectId, fileChange.getFileName());
            fileStorer.applyFileChange(path, committed ? fileChange.getAfter() : fileChange.getBefore());
        }
        // 删除变更日志
        fileStorer.applyFileChange(mutatationPath, null);
        if (committed) receiptMapper.deleteManualReceipt(projectId, operationId);
    }
}
