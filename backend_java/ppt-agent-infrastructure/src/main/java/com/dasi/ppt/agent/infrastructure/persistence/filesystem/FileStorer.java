package com.dasi.ppt.agent.infrastructure.persistence.filesystem;

import com.dasi.ppt.agent.domain.shared.service.PathService;
import com.dasi.ppt.agent.infrastructure.persistence.database.mapper.ProjectMapper;
import jakarta.annotation.PostConstruct;
import lombok.RequiredArgsConstructor;
import lombok.extern.slf4j.Slf4j;
import org.springframework.stereotype.Component;
import org.springframework.transaction.support.TransactionSynchronization;
import org.springframework.transaction.support.TransactionSynchronizationManager;
import org.springframework.util.FileSystemUtils;

import java.io.IOException;
import java.io.UncheckedIOException;
import java.nio.file.DirectoryStream;
import java.nio.file.Files;
import java.nio.file.Path;

import static java.nio.file.LinkOption.NOFOLLOW_LINKS;
import static java.nio.file.StandardCopyOption.ATOMIC_MOVE;

@Slf4j
@Component
@RequiredArgsConstructor
public class FileStorer {

    private final PathService pathService;
    private final ProjectMapper projectMapper;

    @PostConstruct
    void recoverProjectDeletions() {
        // 1. 判断是否存在项目
        Path projectsRoot = pathService.projectsRoot();
        if (Files.notExists(projectsRoot)) {
            return;
        }
        // 2. 遍历所有项目，拿到存在的删除标记
        try (DirectoryStream<Path> entries = Files.newDirectoryStream(projectsRoot, pathService::isDeletingDirectory)) {
            for (Path deletingDirectory : entries) {
                // 3. 从标记中解析出项目
                Path projectDirectory = pathService.projectDirectoryFromDeletion(deletingDirectory);
                String projectId = projectDirectory.getFileName().toString();
                if (projectMapper.existsProject(projectId)) {
                    // 4.1 恢复未提交删除的项目
                    Files.move(deletingDirectory, projectDirectory, ATOMIC_MOVE);
                } else {
                    // 4.2 清理已删除项目的暂存目录
                    FileSystemUtils.deleteRecursively(deletingDirectory);
                }
            }
        } catch (IOException ex) {
            throw new UncheckedIOException(ex);
        }
    }

    public boolean hasDeletingDirectory(String projectId) {
        return Files.exists(pathService.deletingDirectory(projectId), NOFOLLOW_LINKS);
    }

    public void stageProjectDeletion(String projectId) {
        // 1. 拿到两个路径
        Path projectDirectory = pathService.projectDirectory(projectId);
        Path deletingDirectory = pathService.deletingDirectory(projectId);
        if (Files.notExists(projectDirectory, NOFOLLOW_LINKS)) return;
        // 2. 先把原项目做一次备份
        try {
            Files.move(projectDirectory, deletingDirectory, ATOMIC_MOVE);
        } catch (IOException ex) {
            throw new UncheckedIOException(ex);
        }
        // 3. 给事务注册一个回调方法
        TransactionSynchronizationManager.registerSynchronization(new TransactionSynchronization() {
            @Override
            public void afterCompletion(int status) {
                if (status == STATUS_UNKNOWN) {
                    return;
                }
                try {
                    // 3.1 事务回滚则恢复项目
                    if (status == STATUS_ROLLED_BACK) {
                        Files.move(deletingDirectory, projectDirectory, ATOMIC_MOVE);
                    }
                    // 3.2 事务提交则删除备份
                    if (status == STATUS_COMMITTED) {
                        FileSystemUtils.deleteRecursively(deletingDirectory);
                    }
                } catch (IOException ex) {
                    log.error("项目删除暂存目录处理失败，将在下次启动时重试：{}", deletingDirectory, ex);
                }
            }
        });
    }
}
