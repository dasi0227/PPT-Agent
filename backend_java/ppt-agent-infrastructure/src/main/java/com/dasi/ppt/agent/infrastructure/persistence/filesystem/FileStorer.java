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
import java.nio.ByteBuffer;
import java.nio.channels.Channels;
import java.nio.channels.FileChannel;
import java.nio.channels.SeekableByteChannel;
import java.nio.file.*;

import static java.nio.file.LinkOption.NOFOLLOW_LINKS;
import static java.nio.file.StandardCopyOption.ATOMIC_MOVE;
import static java.nio.file.StandardCopyOption.REPLACE_EXISTING;

@Slf4j
@Component
@RequiredArgsConstructor
public class FileStorer {

    private final PathService pathService;
    private final ProjectMapper projectMapper;

    private static final String WRITING_PREFIX = ".writing-";
    private static final String TEMP_SUFFIX = ".tmp";

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

    public byte[] readFile(Path file) {
        try (SeekableByteChannel channel = Files.newByteChannel(file, StandardOpenOption.READ, NOFOLLOW_LINKS)) {
            return Channels.newInputStream(channel).readAllBytes();
        } catch (NoSuchFileException ex) {
            return null;
        } catch (IOException ex) {
            throw new UncheckedIOException(ex);
        }
    }

    public void createFile(Path path, byte[] bytes) {
        try {
            Files.createDirectories(path.getParent());
            FileChannel fileChannel = FileChannel.open(path, StandardOpenOption.CREATE_NEW, StandardOpenOption.WRITE);
            try (fileChannel) {
                writeBytes(fileChannel, bytes);
            } catch (IOException ex) {
                Files.deleteIfExists(path);
                throw ex;
            }
            syncDirectory(path.getParent());
        } catch (IOException ex) {
            throw new UncheckedIOException(ex);
        }
    }

    public void deleteFile(Path path) {
        try {
            Files.deleteIfExists(path);
            syncDirectory(path.getParent());
        } catch (NoSuchFileException ignored) {
            // 项目目录尚未创建时，文件已经是所需的缺失状态
        } catch (IOException ex) {
            throw new UncheckedIOException(ex);
        }
    }

    public void applyFileChange(Path path, byte[] bytes) {
        // 1. 如果是 null，说明是删除文件
        if (bytes == null) {
            deleteFile(path);
            return;
        }
        try {
            // 2. 确保所有目录都是存在的
            Path directory = path.getParent();
            createDirectories(directory);
            // 3. 创建好一个临时文件
            Path temp = Files.createTempFile(directory, WRITING_PREFIX, TEMP_SUFFIX);
            try {
                // 4. 把内容都先写到临时文件
                try (FileChannel fileChannel = FileChannel.open(temp, StandardOpenOption.WRITE)) {
                    writeBytes(fileChannel, bytes);
                }
                // 5. 最后用临时文件原子替换目标文件
                Files.move(temp, path, ATOMIC_MOVE, REPLACE_EXISTING);
            } finally {
                // 6. 清除临时文件
                Files.deleteIfExists(temp);
            }
            // 7. 同步目录项
            syncDirectory(directory);
        } catch (IOException ex) {
            throw new UncheckedIOException(ex);
        }
    }

    private void createDirectories(Path directory) throws IOException {
        if (Files.isDirectory(directory)) return;
        Path parent = directory.toAbsolutePath().getParent();
        // 递归创建完整路径
        createDirectories(parent);
        Files.createDirectories(directory);
        syncDirectory(parent);
    }

    private void writeBytes(FileChannel channel, byte[] bytes) throws IOException {
        ByteBuffer buffer = ByteBuffer.wrap(bytes);
        while (buffer.hasRemaining()) {
            channel.write(buffer);
        }
        channel.force(true);
    }

    private void syncDirectory(Path directory) throws IOException {
        try (FileChannel fileChannel = FileChannel.open(directory, StandardOpenOption.READ)) {
            fileChannel.force(true);
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
