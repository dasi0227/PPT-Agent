package com.dasi.ppt.agent.infrastructure.persistence.filesystem;

import com.dasi.ppt.agent.domain.shared.exception.ErrorCode;
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
import java.util.function.Predicate;

import static com.dasi.ppt.agent.domain.shared.exception.BusinessException.conflict;
import static java.nio.file.LinkOption.NOFOLLOW_LINKS;
import static java.nio.file.StandardCopyOption.ATOMIC_MOVE;

@Slf4j
@Component
public class FileStorer {

    private static final String CREATING = ".creating-";
    private static final String DELETING = ".deleting-";

    public void requireWritable(Path directory) {
        if (Files.exists(directory.resolveSibling(DELETING + directory.getFileName()), NOFOLLOW_LINKS)
                || Files.exists(directory.resolveSibling(CREATING + directory.getFileName()), NOFOLLOW_LINKS)) {
            throw conflict(ErrorCode.PROJECT_BUSY);
        }
        try (DirectoryStream<Path> logs = Files.newDirectoryStream(directory.resolve(".runtime/mutations"), "*.json")) {
            if (logs.iterator().hasNext()) {
                throw conflict(ErrorCode.PROJECT_BUSY);
            }
        } catch (NoSuchFileException ignored) {
            // 尚无文稿写入日志。
        } catch (IOException ex) {
            throw new UncheckedIOException(ex);
        }
    }

    public void createProjectDirectory(Path directory) {
        Path marker = directory.resolveSibling(CREATING + directory.getFileName());
        try {
            Files.createDirectories(directory.getParent());
            if (Files.exists(directory, NOFOLLOW_LINKS)) {
                throw new IOException("项目目录已存在：" + directory);
            }
            Files.createFile(marker);
            try {
                Files.createDirectory(directory);
            } catch (IOException ex) {
                Files.delete(marker);
                throw ex;
            }
            finishAfterTransaction(directory, marker, true);
        } catch (IOException ex) {
            throw new UncheckedIOException(ex);
        }
    }

    public void stageProjectDeletion(Path directory) {
        Path backup = directory.resolveSibling(DELETING + directory.getFileName());
        try {
            if (Files.notExists(directory, NOFOLLOW_LINKS)) {
                return;
            }
            Files.move(directory, backup, ATOMIC_MOVE);
            finishAfterTransaction(directory, backup, false);
        } catch (IOException ex) {
            throw new UncheckedIOException(ex);
        }
    }

    private void finishAfterTransaction(Path directory, Path pending, boolean creating) {
        TransactionSynchronizationManager.registerSynchronization(new TransactionSynchronization() {
            @Override
            public void afterCompletion(int status) {
                if (status == STATUS_UNKNOWN) {
                    return;
                }
                boolean keepProject = creating ? status == STATUS_COMMITTED : status == STATUS_ROLLED_BACK;
                try {
                    finish(directory, pending, creating, keepProject);
                } catch (IOException ex) {
                    log.error("项目目录收尾失败，将在下次启动时重试：{}", directory, ex);
                }
            }
        });
    }

    // 创建标记和待删除目录都保留到事务结束。若进程中断，以 SQLite 已提交的状态决定去留。
    public void recoverProjectDirectories(PathResolver paths, Predicate<String> projectExists) {
        Path root = paths.projectsRoot();
        if (Files.notExists(root)) {
            return;
        }
        try (DirectoryStream<Path> entries = Files.newDirectoryStream(root, ".{creating,deleting}-*")) {
            for (Path pending : entries) {
                String name = pending.getFileName().toString();
                boolean creating = name.startsWith(CREATING);
                String id = name.substring((creating ? CREATING : DELETING).length());
                finish(paths.projectDirectory(id), pending, creating, projectExists.test(id));
            }
        } catch (IOException ex) {
            throw new UncheckedIOException(ex);
        }
    }

    private void finish(Path directory, Path pending, boolean creating, boolean projectExists) throws IOException {
        if (creating) {
            if (projectExists) {
                Files.createDirectories(directory);
            } else {
                FileSystemUtils.deleteRecursively(directory);
            }
            Files.delete(pending);
        } else if (projectExists) {
            Files.move(pending, directory, ATOMIC_MOVE);
        } else {
            FileSystemUtils.deleteRecursively(pending);
        }
    }
}
