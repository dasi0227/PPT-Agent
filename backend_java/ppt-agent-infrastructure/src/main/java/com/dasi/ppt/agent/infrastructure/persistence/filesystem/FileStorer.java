package com.dasi.ppt.agent.infrastructure.persistence.filesystem;

import com.dasi.ppt.agent.domain.shared.service.PathResolver;
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
import java.nio.file.DirectoryStream;
import java.nio.file.Files;
import java.nio.file.NoSuchFileException;
import java.nio.file.Path;
import java.nio.file.StandardOpenOption;

import static java.nio.file.LinkOption.NOFOLLOW_LINKS;
import static java.nio.file.StandardCopyOption.ATOMIC_MOVE;
import static java.nio.file.StandardCopyOption.REPLACE_EXISTING;

@Slf4j
@Component
@RequiredArgsConstructor
public class FileStorer {

    private final PathResolver pathResolver;
    private final ProjectMapper projectMapper;

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
            FileChannel channel = FileChannel.open(path, StandardOpenOption.CREATE_NEW, StandardOpenOption.WRITE);
            try (channel) {
                writeBytes(channel, bytes);
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
            // 项目目录尚未创建时，文件已经是所需的缺失状态。
        } catch (IOException ex) {
            throw new UncheckedIOException(ex);
        }
    }

    // 先写到临时文件，再做一次原子替换
    public void applyFileChange(Path path, byte[] bytes) {
        if (bytes == null) {
            deleteFile(path);
            return;
        }
        Path directory = path.getParent();
        try {
            createDirectories(directory);
            Path temp = Files.createTempFile(directory, ".writing-", ".tmp");
            try {
                try (FileChannel channel = FileChannel.open(temp, StandardOpenOption.WRITE)) {
                    writeBytes(channel, bytes);
                }
                Files.move(temp, path, ATOMIC_MOVE, REPLACE_EXISTING);
            } finally {
                Files.deleteIfExists(temp);
            }
            syncDirectory(directory);
        } catch (IOException ex) {
            throw new UncheckedIOException(ex);
        }
    }

    // 首次写文稿时项目目录可能尚不存在。新目录在父目录中的条目也要先落盘。
    private void createDirectories(Path directory) throws IOException {
        if (Files.isDirectory(directory)) return;
        Path parent = directory.toAbsolutePath().getParent();
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
        try (FileChannel channel = FileChannel.open(directory, StandardOpenOption.READ)) {
            channel.force(true);
        }
    }

    public boolean hasDeletingDirectory(String projectId) {
        return Files.exists(pathResolver.deletingDirectory(projectId), NOFOLLOW_LINKS);
    }

    @PostConstruct
    void recoverProjectDeletions() {
        Path projectsRoot = pathResolver.projectsRoot();
        if (Files.notExists(projectsRoot)) return;
        try (DirectoryStream<Path> entries = Files.newDirectoryStream(projectsRoot, pathResolver::isDeletingDirectory)) {
            for (Path deletingDirectory : entries) {
                Path projectDirectory = pathResolver.projectDirectoryFromDeletion(deletingDirectory);
                String projectId = projectDirectory.getFileName().toString();
                finishProjectDeletion(projectDirectory, deletingDirectory, projectMapper.existsProject(projectId));
            }
        } catch (IOException ex) {
            throw new UncheckedIOException(ex);
        }
    }

    public void stageProjectDeletion(String projectId) {
        Path projectDirectory = pathResolver.projectDirectory(projectId);
        Path deletingDirectory = pathResolver.deletingDirectory(projectId);
        if (Files.notExists(projectDirectory, NOFOLLOW_LINKS)) return;
        try {
            Files.move(projectDirectory, deletingDirectory, ATOMIC_MOVE);
        } catch (IOException ex) {
            throw new UncheckedIOException(ex);
        }

        TransactionSynchronizationManager.registerSynchronization(new TransactionSynchronization() {
            @Override
            public void afterCompletion(int status) {
                if (status == STATUS_UNKNOWN) return;
                try {
                    finishProjectDeletion(projectDirectory, deletingDirectory, status == STATUS_ROLLED_BACK);
                } catch (IOException ex) {
                    log.error("项目删除暂存目录处理失败，将在下次启动时重试：{}", deletingDirectory, ex);
                }
            }
        });
    }

    private void finishProjectDeletion(Path projectDirectory, Path deletingDirectory, boolean restoreProject) throws IOException {
        if (restoreProject) {
            Files.move(deletingDirectory, projectDirectory, ATOMIC_MOVE);
        } else {
            FileSystemUtils.deleteRecursively(deletingDirectory);
        }
    }
}
