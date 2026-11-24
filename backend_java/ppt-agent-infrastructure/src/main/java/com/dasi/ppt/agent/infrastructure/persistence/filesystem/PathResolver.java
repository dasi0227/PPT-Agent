package com.dasi.ppt.agent.infrastructure.persistence.filesystem;

import com.dasi.ppt.agent.domain.shared.exception.ErrorCode;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.stereotype.Component;

import java.io.IOException;
import java.nio.file.Files;
import java.nio.file.InvalidPathException;
import java.nio.file.NoSuchFileException;
import java.nio.file.Path;
import java.util.Locale;
import java.util.Set;

import static com.dasi.ppt.agent.domain.shared.exception.BusinessException.failed;
import static com.dasi.ppt.agent.domain.shared.exception.BusinessException.invalid;
import static com.dasi.ppt.agent.domain.shared.exception.BusinessException.notFound;

@Component
public class PathResolver {

    private static final Set<String> DOCUMENT_EXTENSIONS = Set.of(
            "json", "html", "htm", "css", "md", "txt", "png", "webp", "jpg", "jpeg", "svg", "pdf", "zip"
    );
    private final Path projectsRoot;

    public PathResolver(@Value("${ppt.root}") Path root) {
        this.projectsRoot = root.resolve("projects");
    }

    public Path resolveOpenFile(String projectId, String relativePath) {
        try {
            Path id = Path.of(projectId);
            Path path = Path.of(relativePath);
            if (id.getRoot() != null || id.getNameCount() != 1 || projectId.equals(".") || projectId.equals("..")
                    || path.getRoot() != null || path.normalize().startsWith("..")) {
                throw invalid("需指定项目内的相对文件路径");
            }
            Path root = projectsRoot.toRealPath().resolve(id);
            // 项目目录本身也不能通过符号链接指向其他项目。
            if (!root.equals(root.toRealPath())) {
                throw invalid("项目目录无效");
            }
            Path file = root.resolve(path).toRealPath();
            if (!file.startsWith(root) || !Files.isRegularFile(file)) {
                throw invalid("只能打开本项目内的普通文件");
            }
            String name = file.getFileName().toString().toLowerCase(Locale.ROOT);
            int dot = name.lastIndexOf('.');
            if (dot < 0 || !DOCUMENT_EXTENSIONS.contains(name.substring(dot + 1))) {
                throw invalid("不支持打开此类型的项目文件");
            }
            return file;
        } catch (InvalidPathException e) {
            throw invalid("项目文件路径无效");
        } catch (NoSuchFileException e) {
            throw notFound(ErrorCode.FILE_NOT_FOUND);
        } catch (IOException e) {
            throw failed(ErrorCode.FILE_OPEN_FAILED, e);
        }
    }
}
