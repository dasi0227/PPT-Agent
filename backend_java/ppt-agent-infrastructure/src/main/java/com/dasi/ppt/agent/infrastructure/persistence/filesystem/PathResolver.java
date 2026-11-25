package com.dasi.ppt.agent.infrastructure.persistence.filesystem;

import com.dasi.ppt.agent.domain.shared.exception.ErrorCode;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.stereotype.Component;

import java.io.IOException;
import java.nio.file.Files;
import java.nio.file.NoSuchFileException;
import java.nio.file.Path;
import java.nio.file.attribute.BasicFileAttributes;

import static com.dasi.ppt.agent.domain.shared.exception.BusinessException.failed;
import static com.dasi.ppt.agent.domain.shared.exception.BusinessException.invalid;
import static com.dasi.ppt.agent.domain.shared.exception.BusinessException.notFound;

@Component
public class PathResolver {

    @Value("${ppt.root}")
    private Path dataRoot;

    public Path projectsRoot() {
        return dataRoot.resolve("projects").normalize();
    }

    public Path projectDirectory(String projectId) {
        return projectsRoot().resolve(projectId).normalize();
    }

    public Path resolveFile(String projectId, String relativePath) {
        try {
            Path directory = projectDirectory(projectId).toRealPath();
            Path file = directory.resolve(relativePath).toRealPath();
            if (!file.startsWith(directory) || !Files.readAttributes(file, BasicFileAttributes.class).isRegularFile()) {
                throw invalid("只能打开本项目内的普通文件");
            }
            return file;
        } catch (NoSuchFileException e) {
            throw notFound(ErrorCode.FILE_NOT_FOUND);
        } catch (IOException e) {
            throw failed(ErrorCode.FILE_OPEN_FAILED, e.getMessage());
        }
    }
}
