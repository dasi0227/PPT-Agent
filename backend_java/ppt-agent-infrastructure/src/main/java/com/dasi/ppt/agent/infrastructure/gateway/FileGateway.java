package com.dasi.ppt.agent.infrastructure.gateway;

import com.dasi.ppt.agent.domain.shared.gateway.IFileGateway;
import org.springframework.stereotype.Component;

import java.io.IOException;
import java.io.UncheckedIOException;
import java.nio.ByteBuffer;
import java.nio.channels.Channels;
import java.nio.channels.FileChannel;
import java.nio.channels.SeekableByteChannel;
import java.nio.file.Files;
import java.nio.file.NoSuchFileException;
import java.nio.file.Path;
import java.nio.file.StandardOpenOption;
import java.util.List;
import java.util.stream.Stream;

import static java.nio.file.LinkOption.NOFOLLOW_LINKS;
import static java.nio.file.StandardCopyOption.ATOMIC_MOVE;
import static java.nio.file.StandardCopyOption.REPLACE_EXISTING;

@Component
public class FileGateway implements IFileGateway {

    private static final String WRITING_PREFIX = ".writing-";
    private static final String TEMP_SUFFIX = ".tmp";

    @Override
    public List<String> listFileNames(Path path) {
        try (Stream<Path> files = Files.list(path)) {
            return files.filter(file -> Files.isRegularFile(file, NOFOLLOW_LINKS))
                    .map(file -> file.getFileName().toString())
                    .toList();
        } catch (NoSuchFileException ex) {
            return List.of();
        } catch (IOException ex) {
            throw new UncheckedIOException(ex);
        }
    }

    @Override
    public byte[] readFile(Path path) {
        try (SeekableByteChannel channel = Files.newByteChannel(path, StandardOpenOption.READ, NOFOLLOW_LINKS)) {
            return Channels.newInputStream(channel).readAllBytes();
        } catch (NoSuchFileException ex) {
            return null;
        } catch (IOException ex) {
            throw new UncheckedIOException(ex);
        }
    }

    @Override
    public void createFile(Path path, byte[] content) {
        writeFile(path, content, false);
    }

    @Override
    public void replaceFile(Path path, byte[] content) {
        writeFile(path, content, true);
    }

    @Override
    public void deleteFile(Path path) {
        try {
            Files.deleteIfExists(path);
            syncDirectory(path.toAbsolutePath().getParent());
        } catch (NoSuchFileException ignored) {
            // 父目录已缺失时，文件也已删除。
        } catch (IOException ex) {
            throw new UncheckedIOException(ex);
        }
    }

    private void writeFile(Path file, byte[] content, boolean replaceExisting) {
        Path directory = file.toAbsolutePath().getParent();
        try {
            createDirectories(directory);
            Path temp = Files.createTempFile(directory, WRITING_PREFIX, TEMP_SUFFIX);
            try {
                try (FileChannel fileChannel = FileChannel.open(temp, StandardOpenOption.WRITE)) {
                    ByteBuffer buffer = ByteBuffer.wrap(content);
                    while (buffer.hasRemaining()) {
                        fileChannel.write(buffer);
                    }
                    fileChannel.force(true);
                }
                if (replaceExisting) {
                    Files.move(temp, file, ATOMIC_MOVE, REPLACE_EXISTING);
                } else {
                    // 原子创建正式文件名，同名时由文件系统拒绝。
                    Files.createLink(file, temp);
                }
            } finally {
                Files.deleteIfExists(temp);
            }
            syncDirectory(directory);
        } catch (IOException ex) {
            throw new UncheckedIOException(ex);
        }
    }

    private void createDirectories(Path directory) throws IOException {
        if (Files.isDirectory(directory)) return;
        Path parent = directory.getParent();
        // 递归创建完整路径
        createDirectories(parent);
        Files.createDirectories(directory);
        syncDirectory(parent);
    }

    private void syncDirectory(Path directory) throws IOException {
        try (FileChannel fileChannel = FileChannel.open(directory, StandardOpenOption.READ)) {
            fileChannel.force(true);
        }
    }
}
