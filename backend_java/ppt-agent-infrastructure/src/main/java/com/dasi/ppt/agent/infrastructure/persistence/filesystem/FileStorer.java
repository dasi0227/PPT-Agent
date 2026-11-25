package com.dasi.ppt.agent.infrastructure.persistence.filesystem;

import com.dasi.ppt.agent.domain.shared.exception.ErrorCode;
import com.dasi.ppt.agent.infrastructure.persistence.database.mapper.ProjectMapper;
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
import java.nio.file.NoSuchFileException;
import java.nio.file.Path;

import static com.dasi.ppt.agent.domain.shared.exception.BusinessException.conflict;
import static java.nio.file.LinkOption.NOFOLLOW_LINKS;
import static java.nio.file.StandardCopyOption.ATOMIC_MOVE;

@Slf4j
@Component
@RequiredArgsConstructor
public class FileStorer {

    private static final String CREATING = ".creating-";
    private static final String DELETING = ".deleting-";
    private static final String MUTATING = ".runtime/mutations";

    private final PathResolver pathResolver;
    private final ProjectMapper projectMapper;

    public void requireWritable(Path directory) {
        // 判断是否正在创建中
        Path creatingTag = directory.resolveSibling(CREATING + directory.getFileName());
        if (Files.exists(creatingTag, NOFOLLOW_LINKS)) {
            throw conflict(ErrorCode.PROJECT_BUSY);
        }
        // 判断是否正在删除中
        Path deletingTag = directory.resolveSibling(DELETING + directory.getFileName());
        if (Files.exists(deletingTag, NOFOLLOW_LINKS)) {
            throw conflict(ErrorCode.PROJECT_BUSY);
        }
        // 判断是否正在更改中
        Path mutatingTag = directory.resolve(MUTATING);
        try (DirectoryStream<Path> logs = Files.newDirectoryStream(mutatingTag, "*.json")) {
            if (logs.iterator().hasNext()) {
                throw conflict(ErrorCode.PROJECT_BUSY);
            }
        } catch (NoSuchFileException ignored) {
            // 尚无写入日志。
        } catch (IOException ex) {
            throw new UncheckedIOException(ex);
        }
    }

    public void recoverProjectDirectories() {
        Path projectsRoot = pathResolver.projectsRoot();
        if (Files.notExists(projectsRoot)) {
            return;
        }
        try (DirectoryStream<Path> entries = Files.newDirectoryStream(projectsRoot, ".{creating,deleting}-*")) {
            for (Path pending : entries) {
                String name = pending.getFileName().toString();
                boolean isCreating = name.startsWith(CREATING);
                String projectId = name.substring((isCreating ? CREATING : DELETING).length());
                Path directory = pathResolver.projectDirectory(projectId);
                if (!projectsRoot.equals(directory.getParent())) {
                    throw new IOException("项目恢复标记无效：" + pending);
                }
                alignProjectDirectoryWithDatabase(directory, pending, isCreating, projectMapper.existsProject(projectId));
            }
        } catch (IOException ex) {
            throw new UncheckedIOException(ex);
        }
    }

    public void createProjectDirectory(Path projectDirectory) {
        Path creatingTag = projectDirectory.resolveSibling(CREATING + projectDirectory.getFileName());
        try {
            Files.createDirectories(projectDirectory.getParent());
            if (Files.exists(projectDirectory, NOFOLLOW_LINKS)) {
                throw new IOException("项目目录已存在：" + projectDirectory);
            }
            Files.createFile(creatingTag);
            try {
                Files.createDirectory(projectDirectory);
            } catch (IOException ex) {
                Files.delete(creatingTag);
                throw ex;
            }
            finishAfterTransaction(projectDirectory, creatingTag, true);
        } catch (IOException ex) {
            throw new UncheckedIOException(ex);
        }
    }

    public void stageProjectDeletion(Path directory) {
        Path deletingTag = directory.resolveSibling(DELETING + directory.getFileName());
        try {
            if (Files.notExists(directory, NOFOLLOW_LINKS)) {
                return;
            }
            Files.move(directory, deletingTag, ATOMIC_MOVE);
            finishAfterTransaction(directory, deletingTag, false);
        } catch (IOException ex) {
            throw new UncheckedIOException(ex);
        }
    }

    private void finishAfterTransaction(Path processing, Path pending, boolean isCreating) {
        TransactionSynchronizationManager.registerSynchronization(new TransactionSynchronization() {
            @Override
            public void afterCompletion(int status) {
                if (status == STATUS_UNKNOWN) {
                    return;
                }
                boolean keepProject = isCreating ? status == STATUS_COMMITTED : status == STATUS_ROLLED_BACK;
                try {
                    alignProjectDirectoryWithDatabase(processing, pending, isCreating, keepProject);
                } catch (IOException ex) {
                    log.error("项目目录收尾失败，将在下次启动时重试：{}", processing, ex);
                }
            }
        });
    }

    private void alignProjectDirectoryWithDatabase(Path processing, Path pending, boolean isCreating, boolean keepProject) throws IOException {
        if (isCreating) {
            if (keepProject) {
                Files.createDirectories(processing);
            } else {
                FileSystemUtils.deleteRecursively(processing);
            }
            Files.delete(pending);
        } else {
            if (keepProject) {
                Files.move(pending, processing, ATOMIC_MOVE);
            } else {
                FileSystemUtils.deleteRecursively(pending);
            }
        }
    }
}
