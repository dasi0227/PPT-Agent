package com.dasi.ppt.agent.infrastructure.persistence.filesystem;

import com.dasi.ppt.agent.domain.shared.service.PathResolver;
import com.dasi.ppt.agent.infrastructure.persistence.database.mapper.ProjectMapper;
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
import java.util.List;
import java.util.stream.Stream;

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

    public List<Path> listMutationLogs(String projectId) {
        try (Stream<Path> entries = Files.list(pathResolver.mutationDirectory(projectId))) {
            return entries.filter(path -> path.getFileName().toString().endsWith(".json"))
                    .sorted()
                    .toList();
        } catch (NoSuchFileException ex) {
            return List.of();
        } catch (IOException ex) {
            throw new UncheckedIOException(ex);
        }
    }

    public void writeMutationLog(Path path, byte[] log) {
        Path mutations = path.getParent();
        try {
            Files.createDirectories(mutations);
            syncDirectory(mutations.getParent().getParent());
            syncDirectory(mutations.getParent());
        } catch (IOException ex) {
            throw new UncheckedIOException(ex);
        }
        replaceFile(path, log);
    }

    public void replaceFile(Path path, byte[] bytes) {
        Path directory = path.getParent();
        try {
            if (bytes == null) {
                Files.deleteIfExists(path);
            } else {
                Path temp = Files.createTempFile(directory, ".writing-", ".tmp");
                try {
                    try (FileChannel channel = FileChannel.open(temp, StandardOpenOption.WRITE)) {
                        ByteBuffer buffer = ByteBuffer.wrap(bytes);
                        while (buffer.hasRemaining()) {
                            channel.write(buffer);
                        }
                        channel.force(true);
                    }
                    Files.move(temp, path, ATOMIC_MOVE, REPLACE_EXISTING);
                } finally {
                    Files.deleteIfExists(temp);
                }
            }
            syncDirectory(directory);
        } catch (IOException ex) {
            throw new UncheckedIOException(ex);
        }
    }

    private void syncDirectory(Path directory) throws IOException {
        try (FileChannel channel = FileChannel.open(directory, StandardOpenOption.READ)) {
            channel.force(true);
        }
    }

    public boolean hasPendingFileOperation(String projectId) {
        Path creatingTag = pathResolver.creatingMarker(projectId);
        Path deletingTag = pathResolver.deletingMarker(projectId);
        if (Files.exists(creatingTag, NOFOLLOW_LINKS) || Files.exists(deletingTag, NOFOLLOW_LINKS)) {
            return true;
        }
        try (DirectoryStream<Path> logs = Files.newDirectoryStream(pathResolver.mutationDirectory(projectId), "*.json")) {
            return logs.iterator().hasNext();
        } catch (NoSuchFileException ignored) {
            return false;
        } catch (IOException ex) {
            throw new UncheckedIOException(ex);
        }
    }

    public void recoverProjectDirectories() {
        Path projectsRoot = pathResolver.projectsRoot();
        if (Files.notExists(projectsRoot)) {
            return;
        }
        try (DirectoryStream<Path> entries = Files.newDirectoryStream(projectsRoot, pathResolver::isProjectMarker)) {
            for (Path pending : entries) {
                Path directory = pathResolver.projectDirectoryFromMarker(pending);
                String projectId = directory.getFileName().toString();
                boolean isCreating = pending.equals(pathResolver.creatingMarker(projectId));
                alignProjectDirectoryWithDatabase(directory, pending, isCreating, projectMapper.existsProject(projectId));
            }
        } catch (IOException ex) {
            throw new UncheckedIOException(ex);
        }
    }

    public void createProjectDirectory(String projectId) {
        Path projectDirectory = pathResolver.projectDirectory(projectId);
        Path creatingTag = pathResolver.creatingMarker(projectId);
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

    public void stageProjectDeletion(String projectId) {
        Path directory = pathResolver.projectDirectory(projectId);
        Path deletingTag = pathResolver.deletingMarker(projectId);
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
