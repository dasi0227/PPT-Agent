package com.dasi.ppt.agent.domain.shared.service;

import com.dasi.ppt.agent.domain.shared.exception.ErrorCode;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.stereotype.Component;

import java.io.IOException;
import java.nio.file.Files;
import java.nio.file.NoSuchFileException;
import java.nio.file.Path;
import java.nio.file.attribute.BasicFileAttributes;
import java.util.Set;
import java.util.regex.Matcher;
import java.util.regex.Pattern;

import static com.dasi.ppt.agent.domain.shared.constant.SystemConstant.*;
import static com.dasi.ppt.agent.domain.shared.exception.BusinessException.*;

@Component
public class PathService {

    private static final String DELETING = ".deleting-";
    private static final Set<String> PROJECT_DOCUMENTS = Set.of(META_FILE_NAME, CONSTRAINT_FILE_NAME, OUTLINE_FILE_NAME);
    private static final Pattern SLIDE_FILE = Pattern.compile("(sli_[A-Za-z0-9_-]+)\\.(?:json|html)");

    @Value("${ppt.root}")
    private Path dataRoot;

    public Path projectsRoot() {
        return dataRoot.resolve("projects").normalize();
    }

    public Path projectDirectory(String projectId) {
        return projectsRoot().resolve(projectId).normalize();
    }

    public Path deletingDirectory(String projectId) {
        return projectsRoot().resolve(DELETING + projectId).normalize();
    }

    public String slideIdFromFileName(String fileName) {
        Matcher match = SLIDE_FILE.matcher(fileName);
        return match.matches() ? match.group(1) : null;
    }

    public boolean isProjectFile(String fileName) {
        return fileName != null && (PROJECT_DOCUMENTS.contains(fileName) || SLIDE_FILE.matcher(fileName).matches());
    }

    public boolean isDeletingDirectory(Path path) {
        return path.getFileName().toString().startsWith(DELETING);
    }

    public Path projectDirectoryFromDeletion(Path deletingDirectory) {
        String name = deletingDirectory.getFileName().toString();
        Path directory = projectDirectory(name.substring(DELETING.length()));
        if (!projectsRoot().equals(directory.getParent())) {
            throw illegal("项目删除暂存目录无效：" + deletingDirectory);
        }
        return directory;
    }

    public Path filePathByFileName(String projectId, String fileName) {
        return projectDirectory(projectId).resolve(fileName).normalize();
    }

    public Path resolveDocumentForOpen(String projectId, String fileName) {
        if (!isProjectFile(fileName)) {
            throw invalid("只能打开项目根目录下的文稿文件");
        }
        try {
            Path directory = projectsRoot().toRealPath().resolve(projectId);
            Path file = directory.resolve(fileName).toRealPath();
            if (!directory.equals(file.getParent()) || !isProjectFile(file.getFileName().toString())
                    || !Files.readAttributes(file, BasicFileAttributes.class).isRegularFile()) {
                throw invalid("只能打开项目根目录下的普通文稿文件");
            }
            return file;
        } catch (NoSuchFileException e) {
            throw notFound(ErrorCode.FILE_NOT_FOUND);
        } catch (IOException e) {
            throw failed(ErrorCode.FILE_OPEN_FAILED, e.getMessage());
        }
    }
}
